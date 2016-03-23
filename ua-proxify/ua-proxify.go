package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/url"
	"os"
	"regexp"
	"strings"
	"text/template"
)

type Message map[string]interface{}

func doHMAC(key, data string) (sig string, err error) {
	if bKey, err := base64.StdEncoding.DecodeString(key); err == nil {
		h := hmac.New(sha256.New, bKey)
		h.Write([]byte(data))
		return fmt.Sprintf("%x", h.Sum(nil)), nil
	} else {
		return "", err
	}
}

func ProcessMessage(msg Message) {
	if _, ok := msg["body"]; !ok {
		return
	}

	body, ok := msg["body"].(string)
	if !ok {
		return
	}

	var msgUrl *url.URL
	if _, ok = msg["url"]; ok {
		if _, ok = msg["url"].(string); ok {
			msgUrl, _ = url.Parse(msg["url"].(string))
		}
	}

	attrRe := "\\s*[\"']?\\s*([^\\s\"'>]+)\\s*[\"']?"
	tpl := template.New("url")
	tpl.Funcs(map[string]interface{}{"HMAC": doHMAC})
	tpl = template.Must(tpl.Parse(os.Args[1]))

	body = regexp.MustCompile("<(?:img|style)[^>]+>").ReplaceAllStringFunc(body, func(tag string) string {
		src := regexp.MustCompile("src=" + attrRe).FindStringSubmatch(tag)
		if len(src) > 1 && !strings.HasPrefix(src[1], "data:") {
			resUrl, _ := url.Parse(src[1])
			if resUrl != nil && (msgUrl != nil || resUrl.IsAbs()) {
				if !resUrl.IsAbs() {
					resUrl = msgUrl.ResolveReference(resUrl)
				}
				url := html.UnescapeString(resUrl.String())
				buf := bytes.NewBuffer(nil)
				if err := tpl.Execute(buf, map[string]interface{}{"URL": url, "Message": msg}); err == nil {
					return strings.Replace(tag, src[0], fmt.Sprintf("src=\"%s\"", buf.String()), 1)
				} else {
					log.Printf("Error building new url: %v", err)
				}
			}
		}
		return tag
	})

	msg["body"] = body
}

func main() {
	dec := json.NewDecoder(os.Stdin)
	enc := json.NewEncoder(os.Stdout)
	for {
		msg := make(Message)
		if err := dec.Decode(&msg); err == nil {
			ProcessMessage(msg)
			enc.Encode(msg)
		} else {
			return
		}
	}
}
