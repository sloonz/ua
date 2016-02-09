package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"github.com/sloonz/go-maildir"
	"github.com/sloonz/go-mime-message"
	"github.com/sloonz/go-qprintable"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

var hostname string
var cache *Cache

type Message struct {
	Id          string   `json:"id"`
	Body        string   `json:"body"`
	Title       string   `json:"title"`
	Author      string   `json:"author"`
	AuthorEmail string   `json:"authorEmail"`
	Date        string   `json:"date"`
	References  []string `json:"references"`
	Host        string   `json:"host"`
}

func isDotAtomText(s string) bool {
	pointAllowed := false
	for i := 0; i < len(s); i++ {
		c := s[i]

		// "." is allowed, but not in first position
		// ".." is not allowed
		if c == '.' && pointAllowed {
			pointAllowed = false
			continue
		} else {
			pointAllowed = true
		}

		if c >= 'a' && c <= 'z' {
			continue
		}
		if c >= 'A' && c <= 'Z' {
			continue
		}
		if c >= '0' && c <= '9' {
			continue
		}
		if c == '!' || c == '#' || c == '$' || c == '%' || c == '&' ||
			c == '\'' || c == '*' || c == '+' || c == '-' || c == '/' ||
			c == '=' || c == '?' || c == '^' || c == '_' || c == '`' ||
			c == '{' || c == '|' || c == '}' || c == '~' {
			continue
		}

		return false
	}

	return true
}

func MessageId(id, host string) string {
	// According to RFC 2822:
	// msg-id          =       [CFWS] "<" id-left "@" id-right ">" [CFWS]
	// id-left         =       dot-atom-text / no-fold-quote
	// id-right        =       dot-atom-text / no-fold-literal
	idBuf := bytes.NewBufferString("<")

	if isDotAtomText(id) {
		idBuf.WriteString(id)
	} else {
		// Encode left part as no-fold-quote
		// ASCII 9 (\t), 32 (space), 34 (dquote), 92 (backslash) are escaped with a backslash
		// Non-ASCII and ASCII 0, 10 (\n), 13 (\r) are dropped
		// Other characters are transmitted as-is
		idBuf.WriteByte('"')
		for i := 0; i < len(id); i++ {
			if id[i] == 0 || id[i] == '\r' || id[i] == '\n' || id[i] > 127 {
				// Drop it
			} else if id[i] == '\t' || id[i] == ' ' || id[i] == '"' || id[i] == '\\' {
				idBuf.Write([]byte{'\\', id[i]});
			} else {
				idBuf.WriteByte(id[i]);
			}
		}
		idBuf.WriteByte('"')
	}

	idBuf.WriteByte('@')

	if isDotAtomText(host) {
		idBuf.WriteString(host)
	} else {
		// Encode right part as no-fold-literal
		// ASCII 9 (\t), 32 (space), 91 ([), 92 (backslash) and 93 (]) are escaped with a backslash
		// Non-ASCII and ASCII 0, 10 (\n), 13 (\r) are dropped
		// Other characters are transmitted as-is
		idBuf.WriteByte('[')
		for i := 0; i < len(host); i++ {
			if host[i] == 0 || host[i] == '\r' || host[i] == '\n' || host[i] > 127 {
				// Drop it
			} else if host[i] == '\t' || host[i] == ' ' || host[i] == '[' || host[i] == '\\' || host[i] == ']' {
				idBuf.Write([]byte{'\\', host[i]});
			} else {
				idBuf.WriteByte(host[i]);
			}
		}
		idBuf.WriteByte(']')
	}

	idBuf.WriteByte('>')

	return idBuf.String()
}

func (m *Message) Process(md *maildir.Maildir) error {
	var id string

	if m.Body == "" || m.Title == "" {
		return errors.New("Missing mandatory field")
	}

	if m.Host == "" {
		m.Host = hostname
	}

	if m.AuthorEmail == "" {
		m.AuthorEmail = "noreply@" + m.Host
	}

	if m.Date == "" {
		m.Date = time.Now().UTC().Format(time.RFC1123Z)
	}

	if m.Id != "" {
		id = MessageId(m.Id, m.Host)
		if cache.Get(id) {
			return nil
		} else {
			cache.Set(id)
		}
	}

	mail := message.NewTextMessage(qprintable.UnixTextEncoding, bytes.NewBufferString(m.Body))

	// In a maildir, mails are expected to end with LF line endings. Most softwares are
	// just fine with CRLF line endings, but some (for example Mutt) don’t.
	mail.EOL = "\n"

	mail.SetHeader("Date", m.Date)
	mail.SetHeader("Subject", message.EncodeWord(m.Title))
	mail.SetHeader("From", message.EncodeWord(m.Author)+" <"+m.AuthorEmail+">")
	mail.SetHeader("Content-Type", "text/html; charset=\"UTF-8\"")
	if id != "" {
		mail.SetHeader("Message-Id", id)
	}
	if len(m.References) > 0 {
		refs := ""
		for _, r := range m.References {
			refs += " " + MessageId(r, m.Host)
		}
		mail.SetHeader("References", refs)
	}

	md.CreateMail(mail)

	return nil
}

func main() {
	var rootDir, folder, cacheFile string
	var err error

	flag.StringVar(&rootDir, "root", os.ExpandEnv("$HOME/Maildir"), "path to maildir")
	flag.StringVar(&folder, "folder", "", "maildir folder name to put email (empty for inbox")
	flag.StringVar(&cacheFile, "cache", os.ExpandEnv("$HOME/.cache/maildir-put.cache"),
		"path to store message-ids to drop duplicate messages")

	if flag.Parse(); !flag.Parsed() {
		flag.PrintDefaults()
		os.Exit(1)
	}

	if cache, err = OpenCache(cacheFile); err != nil {
		log.Printf("Can't open cache: %s", err.Error())
		os.Exit(1)
	}

	if hostname, err = os.Hostname(); err != nil {
		log.Print("Can't get hostname: %s", err.Error())
		os.Exit(1)
	}

	md, err := maildir.New(rootDir, true)
	if err != nil {
		log.Print("Can't open maildir: %s", err.Error())
		os.Exit(1)
	}

	for _, subfolder := range strings.Split(folder, "/") {
		if subfolder != "" {
			md, err = md.Child(subfolder, true)
			if err != nil {
				log.Print("Can't open maildir: %s", err.Error())
				os.Exit(1)
			}
		}
	}

	dec := json.NewDecoder(os.Stdin)
	for {
		msg := new(Message)
		err = dec.Decode(msg)
		if err == nil {
			err = msg.Process(md)
		}

		if err == io.EOF {
			break
		} else if err != nil {
			log.Printf("Cannot read input message: %s", err.Error())
		}
	}

	if err = cache.Dump(); err != nil {
		log.Printf("warning: can't dump cache: %s", err.Error())
	}
}
