# maildir-put

`maildir-put` is a tool to put messages in a predefined JSON format
inside a maildir. It also try to detect duplicates and drop them.

## Usage

	message-producer | maildir-put [arguments]

Available arguments:

* *-cache*: path to a cache file used to store message IDs for duplicate
  detection
* *-root*: path to the root maildir directory. Defaults to ~/Maildir.
* *-folder*: maildir folder to put messages. Defaults to "", the inbox.
  The folder separator is "/".

## Installation

	go build && cp maildir-put /usr/local/bin

## Input format

As its input, `maildir-put` takes a stream of JSON dictionaries (not a
list of dictionaries). Each dictionary represents a message. Available
keys are:

* *body*: the body of the message, in HTML. Mandatory.
* *title*: the subject of the message, in text. Mandatory.
* *date*: the date of the message. Optional, defaults to current time. If
  provided, must be RFC 2822 compliant.
* *author*: the name of the author, in text. Optional.
* *authorEmail*: the mail addresse of the author. Optional.
* *id*: an unique identifier for the message. It will be used for the
  creation of the Message-Id header, as well as in duplicates detection. It
  should include three parts: an unique identifier for the application
  (for example: `rss2json`), an unique identifier for the paramenters
  (for example: the feed URL) and an unique identifier for the message
  (for example: an article ID). The identifier for the parameters may be
  omitted if you provide a *host* key and that the host is sufficient to
  identify the parameters. Mandatory for threaded discussions handling and
  duplicates detection, optional else.
* *host*: the domain name of the producer of the message (in general,
  the hostname of the server form where you fetched the information). Used
  in `Message-Id` and `References` headers construction, as well as in
  duplicates detection. Optional, but strongly encouraged for threaded
  discussions handling and duplicates detection.
* *references*: for threaded discussions, *id* of the parent messages. Note
  that *host* must match in the two messages.
* *url*: URL of the message. Used by `ua-inline` to resolve relative
  references.

All strings must be encoded in UTF-8.
