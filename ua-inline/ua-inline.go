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

func fetch(resUrlString string, baseUrl *url.URL) string {
	// Resolve relative url
	resUrl, _ := url.Parse(resUrlString)
	if resUrl == nil || (baseUrl == nil && !resUrl.IsAbs()) {
		return ""
	}

	if !resUrl.IsAbs() {
		resUrl = baseUrl.ResolveReference(resUrl)
	}

	// Test cache
	h := sha256.New()
	h.Write([]byte(resUrl.String()))
	cacheFile := fmt.Sprintf("%s/%x@%s", CacheDir, h.Sum(nil), resUrl.Host)
	data, err := ioutil.ReadFile(cacheFile)
	if err == nil {
		return string(data)
	} else if !os.IsNotExist(err) {
		log.Printf("Can't read cache file %s: %s", cacheFile, err.Error())
	}

	// Cache miss
	resp, err := http.Get(resUrl.String())
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if err != nil {
			log.Printf("Error downloading %s: %s", resUrl.String(), err.Error())
		} else {
			log.Printf("Error downloading %s: %s", resUrl.String(), resp.Status)
		}
		return ""
	}

	data, err = ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.Printf("Error downloading %s: %s", resUrl.String(), err.Error())
		return ""
	}

	// Transform to data: URI scheme
	var mimetype string
	if _, ok := resp.Header["Content-Type"]; ok {
		mimetype = resp.Header["Content-Type"][0]
	} else {
		mimetype = http.DetectContentType(data)
	}
	if strings.Contains(mimetype, ";") {
		mimetype = mimetype[:strings.Index(mimetype, ";")]
	}
	data = []byte(fmt.Sprintf("data:%s;base64,%s", mimetype, base64.StdEncoding.EncodeToString(data)))

	// Write to cache
	if err = ioutil.WriteFile(cacheFile, data, os.FileMode(0644)); err != nil {
		log.Printf("Can't write cache file %s: %s", cacheFile, err.Error())
	}

	return string(data)
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


	attrRe := "\\s*[\"']?\\s*([^\\s\"'>]+)\\s*[\"']?"
	body = regexp.MustCompile("<img[^>]+>").ReplaceAllStringFunc(body, func(img string) string {
		src := regexp.MustCompile("src="+attrRe).FindStringSubmatch(img)
		if len(src) > 1 && !strings.HasPrefix(src[1], "data:") {
			data := fetch(html.UnescapeString(src[1]), msgUrl)
			if data != "" {
				return strings.Replace(img, src[0], "src=\""+data+"\"", 1)
			}
		}
		return img
	})

	msg["body"] = body

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
