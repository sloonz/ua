# rss2json

`rss2json` is a simple tool intended to be used with `maildir-put` and `ggs`. It is used to convert any RSS or Atom feed into a stream of messages usable by `maildir-put`.

## Usage

	rss2json feed-url

	rss2json -url=feed-url < feed-from-stdin

## Dependencies

* libxml
* Optional: python and feedparser for parsing of ill-formed feeds

## Installation

	go build && cp rss2json /usr/local/bin
