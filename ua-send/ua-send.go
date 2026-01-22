package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"os"
	"strconv"
	"strings"

	"github.com/sloonz/ua/uamessage"
)

func readResponseLine(tp *textproto.Conn) (int, string, bool, error) {
	line, err := tp.ReadLine()
	if err != nil {
		return 0, "", false, err
	}
	if len(line) < 3 {
		return 0, "", false, fmt.Errorf("invalid response: %q", line)
	}
	code, err := strconv.Atoi(line[:3])
	if err != nil {
		return 0, "", false, fmt.Errorf("invalid response: %q", line)
	}
	more := len(line) > 3 && line[3] == '-'
	text := ""
	if len(line) > 4 {
		text = line[4:]
	}
	return code, text, more, nil
}

func lmtpCmd(tp *textproto.Conn, expect int, format string, args ...interface{}) error {
	id, err := tp.Cmd(format, args...)
	if err != nil {
		return err
	}
	tp.StartResponse(id)
	defer tp.EndResponse(id)
	if _, _, err := tp.ReadResponse(expect); err != nil {
		return err
	}
	return nil
}

func sendLMTP(network, addr, helo string, startTLS bool, insecure bool, from string, to []string, msg []byte) error {
	conn, err := net.Dial(network, addr)
	if err != nil {
		return err
	}
	defer func() {
		_ = conn.Close()
	}()

	tp := textproto.NewConn(conn)
	defer func() {
		_ = tp.Close()
	}()

	code, _, _, err := readResponseLine(tp)
	if err != nil {
		return err
	}
	if code != 220 {
		return fmt.Errorf("lmtp: unexpected greeting %d", code)
	}

	if err := lmtpCmd(tp, 250, "LHLO %s", helo); err != nil {
		return err
	}

	if startTLS {
		if network != "tcp" {
			return errors.New("LMTP STARTTLS is only supported over tcp")
		}
		if err := lmtpCmd(tp, 220, "STARTTLS"); err != nil {
			return err
		}
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return err
		}
		tlsConn := tls.Client(conn, &tls.Config{ServerName: host, InsecureSkipVerify: insecure})
		if err := tlsConn.Handshake(); err != nil {
			return err
		}
		conn = tlsConn
		tp = textproto.NewConn(conn)
		if err := lmtpCmd(tp, 250, "LHLO %s", helo); err != nil {
			return err
		}
	}

	if err := lmtpCmd(tp, 250, "MAIL FROM:<%s>", from); err != nil {
		return err
	}
	for _, rcpt := range to {
		if err := lmtpCmd(tp, 250, "RCPT TO:<%s>", rcpt); err != nil {
			return err
		}
	}

	id, err := tp.Cmd("DATA")
	if err != nil {
		return err
	}
	tp.StartResponse(id)
	if _, _, err := tp.ReadResponse(354); err != nil {
		tp.EndResponse(id)
		return err
	}

	w := tp.DotWriter()
	if _, err := w.Write(msg); err != nil {
		w.Close()
		tp.EndResponse(id)
		return err
	}
	if err := w.Close(); err != nil {
		tp.EndResponse(id)
		return err
	}
	tp.EndResponse(id)

	for {
		respCode, respText, more, err := readResponseLine(tp)
		if err != nil {
			return err
		}
		if respCode < 200 || respCode >= 300 {
			return fmt.Errorf("lmtp: delivery failed: %d %s", respCode, respText)
		}
		if !more {
			break
		}
	}

	_ = lmtpCmd(tp, 221, "QUIT")

	return nil
}

func sendSMTP(addr string, auth smtp.Auth, insecure bool, from string, to []string, msg []byte) error {
	c, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer c.Close()

	if err = c.Hello("localhost"); err != nil {
		return err
	}
	if ok, _ := c.Extension("STARTTLS"); ok {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return err
		}
		config := &tls.Config{ServerName: host, InsecureSkipVerify: insecure}
		if err = c.StartTLS(config); err != nil {
			return err
		}
	}
	if auth != nil {
		if ok, _ := c.Extension("AUTH"); !ok {
			return errors.New("smtp: server doesn't support AUTH")
		}
		if err = c.Auth(auth); err != nil {
			return err
		}
	}
	if err = c.Mail(from); err != nil {
		return err
	}
	for _, addr := range to {
		if err = c.Rcpt(addr); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err = w.Write(msg); err != nil {
		return err
	}
	if err = w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

func parseAddressList(raw string) ([]string, error) {
	if raw == "" {
		return nil, nil
	}

	addrs, err := mail.ParseAddressList(raw)
	if err != nil {
		candidate := strings.TrimSpace(raw)
		if candidate != "" && !strings.ContainsAny(candidate, ",<>") && !strings.Contains(candidate, " ") {
			return []string{candidate}, nil
		}
		return nil, err
	}

	out := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		out = append(out, addr.Address)
	}

	return out, nil
}

func main() {
	var server, username, password string
	var lmtp bool
	var lmtpNetwork string
	var fromOverride string
	var defaultFrom string
	var toList string
	var folder string
	var folderHeader string
	var startTLS bool
	var insecure bool

	flag.StringVar(&server, "server", "127.0.0.1:25", "SMTP/LMTP server address (host:port for tcp, path for unix)")
	flag.BoolVar(&lmtp, "lmtp", false, "use LMTP instead of SMTP")
	flag.StringVar(&lmtpNetwork, "lmtp-network", "tcp", "network for LMTP (tcp or unix)")
	flag.StringVar(&username, "user", "", "SMTP username")
	flag.StringVar(&password, "password", "", "SMTP password")
	flag.StringVar(&fromOverride, "from", "", "override sender address")
	flag.StringVar(&defaultFrom, "default-from", "", "sender address to use when missing (defaults to -to)")
	flag.StringVar(&toList, "to", "", "recipient addresses")
	flag.StringVar(&folder, "folder", "", "folder name to set in header (empty to skip)")
	flag.StringVar(&folderHeader, "folder-header", "x-fileinto", "header to set with -folder")
	flag.BoolVar(&startTLS, "starttls", false, "enable LMTP STARTTLS")
	flag.BoolVar(&insecure, "insecure", false, "skip TLS certificate verification")

	if flag.Parse(); !flag.Parsed() {
		flag.PrintDefaults()
		os.Exit(1)
	}

	if lmtp {
		if username != "" || password != "" {
			log.Fatal("SMTP auth flags are not supported with LMTP")
		}
	}

	var auth smtp.Auth
	if !lmtp && username != "" {
		host, _, err := net.SplitHostPort(server)
		if err != nil {
			log.Fatalf("Invalid -server value: %s", err.Error())
		}
		auth = smtp.PlainAuth("", username, password, host)
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "localhost"
	}

	dec := json.NewDecoder(os.Stdin)
	enc := json.NewEncoder(os.Stdout)
	for {
		msg := new(uamessage.Message)
		err = dec.Decode(msg)
		if err == nil {
			var recipients []string
			effectiveFromOverride := fromOverride
			if effectiveFromOverride == "" && msg.AuthorEmail == "" {
				defaultFromValue := defaultFrom
				defaultFromSource := "-default-from"
				if defaultFromValue == "" {
					defaultFromValue = toList
					defaultFromSource = "-to"
				}
				if defaultFromValue != "" {
					defaultAddrs, parseErr := parseAddressList(defaultFromValue)
					if parseErr != nil {
						err = fmt.Errorf("invalid %s value: %w", defaultFromSource, parseErr)
					} else if len(defaultAddrs) > 0 {
						effectiveFromOverride = defaultAddrs[0]
					}
				}
			}
			if err == nil {
				mailMsg, fromAddr, buildErr := uamessage.Build(msg, uamessage.BuildOptions{
					Folder:            folder,
					FolderHeader:      strings.TrimSpace(folderHeader),
					FromOverride:      effectiveFromOverride,
					To:                toList,
					RequireSender:     true,
					FillDateIfMissing: true,
				})
				if buildErr != nil {
					err = buildErr
				} else {
					var parseErr error
					recipients, parseErr = parseAddressList(toList)
					if parseErr != nil {
						err = fmt.Errorf("invalid -to value: %w", parseErr)
					} else if len(recipients) == 0 {
						err = errors.New("Missing recipients")
					}
				}
				if err == nil {
					mailBytes, readErr := io.ReadAll(mailMsg)
					if readErr != nil {
						err = readErr
					} else if lmtp {
						err = sendLMTP(lmtpNetwork, server, hostname, startTLS, insecure, fromAddr, recipients, mailBytes)
					} else {
						err = sendSMTP(server, auth, insecure, fromAddr, recipients, mailBytes)
					}
					if err == nil {
						if encErr := enc.Encode(msg); encErr != nil {
							err = encErr
						}
					}
				}
			}
		}

		if err == io.EOF {
			break
		} else if err != nil {
			log.Printf("Cannot send message: %s", err.Error())
		}
	}
}
