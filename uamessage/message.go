package uamessage

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	message "github.com/sloonz/go-mime-message"
	"github.com/sloonz/go-qprintable"
)

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

type BuildOptions struct {
	Folder            string
	FolderHeader      string
	FromOverride      string
	To                string
	RequireSender     bool
	FillDateIfMissing bool
	EOL               string
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

func formatDate(date string) string {
	parsedDate, err := time.Parse(time.RFC3339, date)
	if err != nil {
		return date
	}

	return parsedDate.Format(time.RFC1123Z)
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

func formatAddress(name, email string) string {
	if name == "" {
		return email
	}

	return message.EncodeWord(name) + " <" + email + ">"
}

func Build(msg *Message, opts BuildOptions) (*message.Message, string, error) {
	if msg.Body == "" || msg.Title == "" {
		return nil, "", errors.New("Missing mandatory field")
	}

	fromAddr := msg.AuthorEmail
	if opts.FromOverride != "" {
		fromAddr = opts.FromOverride
	}
	if opts.RequireSender && fromAddr == "" {
		return nil, "", errors.New("Missing sender address")
	}

	var id string
	if msg.Id != "" {
		id = MessageId(msg.Id, msg.Host)
	}

	rootContentType := "text/html; charset=\"UTF-8\""

	bodyPart := message.NewTextMessage(qprintable.UnixTextEncoding, bytes.NewBufferString(msg.Body))
	bodyPart.SetHeader("Content-Type", rootContentType)

	var mailMsg *message.Message
	if msg.Attachments == nil {
		mailMsg = bodyPart
	} else {
		ctBuf := bytes.NewBufferString("")
		encNoFoldQuote(rootContentType, ctBuf, false)
		rootPart := message.NewMultipartMessageParams("related", "",
			map[string]string{"type": ctBuf.String()})

		rootPart.AddPart(bodyPart)
		for _, attachment := range msg.Attachments {
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

		mailMsg = &rootPart.Message
	}

	dateValue := msg.Date
	if dateValue == "" {
		if opts.FillDateIfMissing {
			dateValue = time.Now().UTC().Format(time.RFC1123Z)
		}
	} else {
		dateValue = formatDate(dateValue)
	}

	mailMsg.SetHeader("Date", dateValue)
	mailMsg.SetHeader("Subject", message.EncodeWord(msg.Title))
	mailMsg.SetHeader("From", formatAddress(msg.Author, fromAddr))
	if id != "" {
		mailMsg.SetHeader("Message-Id", id)
	}
	if len(msg.References) > 0 {
		refs := ""
		for _, r := range msg.References {
			refs += " " + MessageId(r, msg.Host)
		}
		mailMsg.SetHeader("References", refs)
	}
	if opts.To != "" {
		mailMsg.SetHeader("To", opts.To)
	}
	if opts.Folder != "" && opts.FolderHeader != "" {
		mailMsg.SetHeader(opts.FolderHeader, opts.Folder)
	}
	if opts.EOL != "" {
		mailMsg.EOL = opts.EOL
	}

	return mailMsg, fromAddr, nil
}
