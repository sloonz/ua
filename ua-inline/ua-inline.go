package main

// TODO:
//  Parallelize
//  Manage cache entries lifetime

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"regexp"
	"strings"
)

type Message map[string]interface{}

var CacheDir string

func hash(name string) string {
	h := sha256.New()
	h.Write([]byte(name))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func fetch(resUrlString string, baseUrl *url.URL) (data []byte, contentType string) {
	var err error

	// Resolve relative url
	resUrl, _ := url.Parse(resUrlString)
	if resUrl == nil || (baseUrl == nil && !resUrl.IsAbs()) {
		return nil, ""
	}

	if !resUrl.IsAbs() {
		resUrl = baseUrl.ResolveReference(resUrl)
	}

	// Test cache
	h := hash(resUrl.String())
	dataCacheFile := fmt.Sprintf("%s/data-%x@%s", CacheDir, h, resUrl.Host)
	typeCacheFile := fmt.Sprintf("%s/type-%x@%s", CacheDir, h, resUrl.Host)
	data, err = ioutil.ReadFile(dataCacheFile)
	if err == nil {
		var bContentType []byte
		bContentType, err = ioutil.ReadFile(typeCacheFile)
		contentType = string(bContentType)
	}
	if err == nil {
		return
	} else if !os.IsNotExist(err) {
		log.Printf("Can't read cache file %s or %s: %s", dataCacheFile, typeCacheFile, err.Error())
	}

	// Cache miss
	resp, err := http.Get(resUrl.String())
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if err != nil {
			log.Printf("Error downloading %s: %s", resUrl.String(), err.Error())
		} else {
			log.Printf("Error downloading %s: %s", resUrl.String(), resp.Status)
		}
		return nil, ""
	}

	data, err = ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.Printf("Error downloading %s: %s", resUrl.String(), err.Error())
		return nil, ""
	}

	// Get type
	if _, ok := resp.Header["Content-Type"]; ok {
		contentType = resp.Header["Content-Type"][0]
	} else {
		contentType = http.DetectContentType(data)
	}

	// Write to cache
	if err = ioutil.WriteFile(dataCacheFile, data, os.FileMode(0644)); err != nil {
		log.Printf("Can't write cache file %s: %s", dataCacheFile, err.Error())
	}
	if err = ioutil.WriteFile(typeCacheFile, []byte(contentType), os.FileMode(0644)); err != nil {
		log.Printf("Can't write cache file %s: %s", typeCacheFile, err.Error())
	}

	return
}

func ProcessMessage(msg Message, ch chan Message) {
	if _, ok := msg["body"]; !ok {
		ch <- msg
		return
	}

	body, ok := msg["body"].(string)
	if !ok {
		ch <- msg
		return
	}

	var msgUrl *url.URL
	if _, ok = msg["url"]; ok {
		if _, ok = msg["url"].(string); ok {
			msgUrl, _ = url.Parse(msg["url"].(string))
		}
	}

	var attachments []map[string]string
	attrRe := "\\s*[\"']?\\s*([^\\s\"'>]+)\\s*[\"']?"

	// Inline <img> as attachment
	body = regexp.MustCompile("<img[^>]+>").ReplaceAllStringFunc(body, func(img string) string {
		src := regexp.MustCompile("src="+attrRe).FindStringSubmatch(img)
		if len(src) > 1 && !strings.HasPrefix(src[1], "data:") {
			cid := hash(src[1])
			filename := regexp.MustCompile("/([^/?]+)(\\?|$)").FindStringSubmatch(src[1])
			data, mimeType := fetch(html.UnescapeString(src[1]), msgUrl)
			if data != nil {
				attachment := map[string]string {
					"cid": cid,
					"mimeType": mimeType,
					"data": base64.StdEncoding.EncodeToString(data)}
				if filename != nil {
					attachment["filename"] = filename[1]
				}
				attachments = append(attachments, attachment)
				return strings.Replace(img, src[0], fmt.Sprintf("src=\"cid:%s\"", cid), 1)
			}
		}
		return img
	})

	// Inline <style src>
	body = regexp.MustCompile("<style[^>]+>").ReplaceAllStringFunc(body, func(style string) string {
		src := regexp.MustCompile("src="+attrRe).FindStringSubmatch(style)
		if len(src) > 1 && !strings.HasPrefix(src[1], "data:") {
			data, mimeType := fetch(html.UnescapeString(src[1]), msgUrl)
			if data != nil {
				newSrc := fmt.Sprintf("src=\"data:%s;base64,%s\"", mimeType, base64.StdEncoding.EncodeToString(data))
				return strings.Replace(style, src[0], newSrc, 1)
			}
		}
		return style
	})

	msg["body"] = body

	if attachments != nil {
		msg["attachments"] = attachments
	}

	ch <- msg
}

func main() {
	user, err := user.Current()
	if err != nil {
		log.Fatalf("Can't find current user: %s", err.Error())
	}

	CacheDir = user.HomeDir + "/.cache/ua-inline/"
	if err = os.MkdirAll(CacheDir, os.FileMode(0755)); err != nil {
		log.Fatalf("Can't create cache dir: %s", err.Error())
	}

	msgCount := 0
	ch := make(chan Message)

	dec := json.NewDecoder(os.Stdin)
	for {
		msg := make(Message)
		err = dec.Decode(&msg)
		if err == nil {
			go ProcessMessage(msg, ch)
			msgCount++
		}

		if err == io.EOF {
			break
		} else if err != nil {
			log.Printf("Cannot read input message: %s", err.Error())
		}
	}

	enc := json.NewEncoder(os.Stdout)
	for msgCount > 0 {
		if err = enc.Encode(<-ch); err != nil {
			log.Printf("Cannot encode message: %s", err.Error())
		}
		msgCount--
	}
}
