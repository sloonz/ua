package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
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
var cache Cache

type Attachment struct {
	CID      string `json:"cid"`
	MimeType string `json:"mimeType"`
	Data     []byte `json:"data"`
	Filename string `json:"filename"`
}

type Message struct {
	Id          string       `json:"id"`
	Body        string       `json:"body"`
	Title       string       `json:"title"`
	Author      string       `json:"author"`
	AuthorEmail string       `json:"authorEmail"`
	Date        string       `json:"date"`
	References  []string     `json:"references"`
	Host        string       `json:"host"`
	Attachments []Attachment `json:"attachments"`
}

func isAtomText(s string, allowDot bool) bool {
	if s == "" {
		return false
	}

	pointAllowed := false
	for i := 0; i < len(s); i++ {
		c := s[i]

		// "." is allowed, but not in first position
		// ".." is not allowed
		if c == '.' && pointAllowed && allowDot {
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

// allowDot=true is for no-fold-quote ; allowDot=fales is for quoted-string
func encNoFoldQuote(s string, buf *bytes.Buffer, allowDot bool) {
	if isAtomText(s, allowDot) {
		buf.WriteString(s)
	} else {
		// Encode left part as no-fold-quote
		// ASCII 9 (\t), 32 (space), 34 (dquote), 92 (backslash) are escaped with a backslash
		// Non-ASCII and ASCII 0, 10 (\n), 13 (\r) are dropped
		// Other characters are transmitted as-is
		buf.WriteByte('"')
		for i := 0; i < len(s); i++ {
			if s[i] == 0 || s[i] == '\r' || s[i] == '\n' || s[i] > 127 {
				// Drop it
			} else if s[i] == '\t' || s[i] == ' ' || s[i] == '"' || s[i] == '\\' {
				buf.Write([]byte{'\\', s[i]})
			} else {
				buf.WriteByte(s[i])
			}
		}
		buf.WriteByte('"')
	}
}

func encNoFoldLiteral(s string, buf *bytes.Buffer) {
	if isAtomText(s, true) {
		buf.WriteString(s)
	} else {
		// Encode right part as no-fold-literal
		// ASCII 9 (\t), 32 (space), 91 ([), 92 (backslash) and 93 (]) are escaped with a backslash
		// Non-ASCII and ASCII 0, 10 (\n), 13 (\r) are dropped
		// Other characters are transmitted as-is
		buf.WriteByte('[')
		for i := 0; i < len(s); i++ {
			if s[i] == 0 || s[i] == '\r' || s[i] == '\n' || s[i] > 127 {
				// Drop it
			} else if s[i] == '\t' || s[i] == ' ' || s[i] == '[' || s[i] == '\\' || s[i] == ']' {
				buf.Write([]byte{'\\', s[i]})
			} else {
				buf.WriteByte(s[i])
			}
		}
		buf.WriteByte(']')
	}
}

func MessageId(id, host string) string {
	// According to RFC 2822:
	// msg-id          =       [CFWS] "<" id-left "@" id-right ">" [CFWS]
	// id-left         =       dot-atom-text / no-fold-quote
	// id-right        =       dot-atom-text / no-fold-literal
	idBuf := bytes.NewBufferString("<")
	encNoFoldQuote(id, idBuf, true)
	idBuf.WriteByte('@')
	encNoFoldLiteral(host, idBuf)
	idBuf.WriteByte('>')

	return idBuf.String()
}

func (m *Message) Process(md *maildir.Maildir) error {
	var id string
	var mail *message.Message

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
		if cache.Getset(m.Id, m.Host, id) {
			return nil
		}
	}

	rootContentType := "text/html; charset=\"UTF-8\""

	bodyPart := message.NewTextMessage(qprintable.UnixTextEncoding, bytes.NewBufferString(m.Body))
	bodyPart.SetHeader("Content-Type", rootContentType)

	if m.Attachments == nil {
		mail = bodyPart
	} else {
		ctBuf := bytes.NewBufferString("")
		encNoFoldQuote(rootContentType, ctBuf, false)
		rootPart := message.NewMultipartMessageParams("related", "",
			map[string]string{"type": ctBuf.String()})

		rootPart.AddPart(bodyPart)
		for _, attachment := range m.Attachments {
			attPart := message.NewBinaryMessage(bytes.NewBuffer(attachment.Data))
			attPart.SetHeader("Content-ID", fmt.Sprintf("<%s>", attachment.CID))
			attPart.SetHeader("Content-Type", attachment.MimeType)
			if attachment.Filename == "" {
				attPart.SetHeader("Content-Disposition", "inline")
			} else {
				fnBuf := bytes.NewBufferString("")
				encNoFoldQuote(attachment.Filename, fnBuf, false)
				attPart.SetHeader("Content-Description", attachment.Filename)
				attPart.SetHeader("Content-Disposition", fmt.Sprintf("inline; filename=%s", fnBuf.String()))
			}
			rootPart.AddPart(attPart)
		}

		mail = &rootPart.Message
	}

	// In a maildir, mails are expected to end with LF line endings. Most softwares are
	// just fine with CRLF line endings, but some (for example Mutt) don’t.
	mail.EOL = "\n"
	mail.SetHeader("Date", m.Date)
	mail.SetHeader("Subject", message.EncodeWord(m.Title))
	mail.SetHeader("From", message.EncodeWord(m.Author)+" <"+m.AuthorEmail+">")
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
	var rootDir, folder string
	var err error

	flag.StringVar(&rootDir, "root", os.ExpandEnv("$HOME/Maildir"), "path to maildir")
	flag.StringVar(&folder, "folder", "", "maildir folder name to put email (empty for inbox)")
	flag.StringVar(&cache.path, "cache", os.ExpandEnv("$HOME/.cache/maildir-put.cache"),
		"path to store message-ids to drop duplicate messages")
	flag.BoolVar(&cache.useRedis, "redis", false, "use redis for cache storage")
	flag.StringVar(&cache.redisOptions.Addr, "redis-addr", "127.0.0.1:6379", "redis address")
	flag.Int64Var(&cache.redisOptions.DB, "redis-db", 0, "redis base")
	flag.StringVar(&cache.redisOptions.Password, "redis-password", "", "redis password")

	if flag.Parse(); !flag.Parsed() {
		flag.PrintDefaults()
		os.Exit(1)
	}

	if err = cache.OpenCache(); err != nil {
		log.Fatalf("Can't open cache: %s", err.Error())
	}

	if hostname, err = os.Hostname(); err != nil {
		log.Fatalf("Can't get hostname: %s", err.Error())
	}

	md, err := maildir.New(rootDir, true)
	if err != nil {
		log.Fatalf("Can't open maildir: %s", err.Error())
	}

	for _, subfolder := range strings.Split(folder, "/") {
		if subfolder != "" {
			md, err = md.Child(subfolder, true)
			if err != nil {
				log.Fatalf("Can't open maildir: %s", err.Error())
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
