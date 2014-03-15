package main

import (
	"encoding/json"
	"fmt"
	"github.com/sloonz/cfeedparser"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

func firstNonEmpty(s ...string) string {
	var val string
	for _, val = range s {
		if val != "" {
			break
		}
	}
	return val
}

func getRFC822Date(e *feedparser.Entry) string {
	emptyTime := time.Time{}
	if e.PublicationDateParsed != emptyTime {
		return e.PublicationDateParsed.Format(time.RFC1123Z)
	}
	if e.ModificationDateParsed != emptyTime {
		return e.ModificationDateParsed.Format(time.RFC1123Z)
	}
	if e.PublicationDate != "" {
		return e.PublicationDate
	}
	if e.ModificationDate != "" {
		return e.ModificationDate
	}
	return time.Now().UTC().Format(time.RFC1123Z)
}

var convertEOLReg = regexp.MustCompile("\r\n?")

func convertEOL(s string) string {
	return convertEOLReg.ReplaceAllString(s, "\n")
}

func process(rawUrl string) error {
	url_, err := url.Parse(rawUrl)
	if err != nil {
		return err
	}

	feed, err := feedparser.ParseURL(url_)
	if err != nil {
		return err
	}

	for _, entry := range feed.Entries {
		body := convertEOL(firstNonEmpty(entry.Content, entry.Summary))
		body += "\n<p><small><a href=\"" + entry.Link + "\">View post</a></small></p>\n"

		jsonEntry := make(map[string]string)
		jsonEntry["id"] = firstNonEmpty(entry.Id, entry.Link, entry.PublicationDate+":"+entry.Title) + ":" + rawUrl
		jsonEntry["title"] = strings.TrimSpace(entry.Title)
		jsonEntry["body"] = body
		jsonEntry["author"] = strings.TrimSpace(firstNonEmpty(entry.Author.Name, entry.Author.Uri, entry.Author.Text))
		jsonEntry["authorAddress"] = strings.TrimSpace(entry.Author.Email)
		jsonEntry["date"] = getRFC822Date(&entry)
		jsonEntry["host"] = url_.Host
		if entry.Link == "" {
			jsonEntry["url"] = url_.String()
		} else {
			jsonEntry["url"] = entry.Link
		}


		encodedEntry, err := json.Marshal(jsonEntry)
		if err != nil {
			return err
		}

		fmt.Printf("%s\n", string(encodedEntry))
	}

	return nil
}

func main() {
	err := process(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't process feed: %s\n", err.Error())
		os.Exit(1)
	}
}
