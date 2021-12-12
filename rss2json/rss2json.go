package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/sloonz/cfeedparser"
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

func getDate(e *feedparser.Entry) string {
	emptyTime := time.Time{}
	if e.PublicationDateParsed != emptyTime {
		return e.PublicationDateParsed.Format(time.RFC3339)
	}
	if e.ModificationDateParsed != emptyTime {
		return e.ModificationDateParsed.Format(time.RFC3339)
	}
	if e.PublicationDate != "" {
		return e.PublicationDate
	}
	if e.ModificationDate != "" {
		return e.ModificationDate
	}
	return time.Now().UTC().Format(time.RFC3339)
}

var convertEOLReg = regexp.MustCompile("\r\n?")

func convertEOL(s string) string {
	return convertEOLReg.ReplaceAllString(s, "\n")
}

func process(rawFeedUrl, rawBaseUrl string) error {
	feedUrl, err := url.Parse(rawFeedUrl)
	if err != nil {
		return err
	}

	baseUrl, err := url.Parse(rawBaseUrl)
	if err != nil {
		return err
	}

	var feed *feedparser.Feed
	if feedUrl.Scheme != "stdin" {
		feed, err = feedparser.ParseURL(feedUrl)
	} else {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		feed, err = feedparser.ParseString(string(data))
	}

	if err != nil {
		return err
	}

	for _, entry := range feed.Entries {
		body := convertEOL(firstNonEmpty(entry.Content, entry.Summary))
		body += "\n<p><small><a href=\"" + entry.Link + "\">View post</a></small></p>\n"

		linkUrl, err := url.Parse(entry.Link)
		linkHost := ""
		if err == nil {
			linkHost = linkUrl.Host
		}

		jsonEntry := make(map[string]string)
		jsonEntry["id"] = firstNonEmpty(entry.Id, entry.Link, entry.PublicationDate+":"+entry.Title) + ":" + rawBaseUrl
		jsonEntry["title"] = strings.TrimSpace(entry.Title)
		jsonEntry["body"] = body
		jsonEntry["author"] = strings.TrimSpace(firstNonEmpty(entry.Author.Name, entry.Author.Uri, entry.Author.Text))
		jsonEntry["authorAddress"] = strings.TrimSpace(entry.Author.Email)
		jsonEntry["date"] = getDate(&entry)
		jsonEntry["host"] = firstNonEmpty(baseUrl.Host, linkHost)
		if entry.Link == "" {
			jsonEntry["url"] = baseUrl.String()
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
	baseUrlFlag := flag.String("url", "", "override feed url, useful for feeds given on stdin")
	flag.Parse()

	feedUrl := "stdin:"
	if flag.NArg() > 0 {
		feedUrl = flag.Args()[0]
	}

	baseUrl := feedUrl
	if *baseUrlFlag != "" {
		baseUrl = *baseUrlFlag
	}

	err := process(feedUrl, baseUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't process feed: %s\n", err.Error())
		os.Exit(1)
	}
}
