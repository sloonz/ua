# ua-inline -- Inline HTML resources

This is a simple filter intended to be used before `maildir-put`. It
replaces external images inside the body of the message by their content
(using `data:` scheme).

If the body contains relative references, it tries to resolve them using
the `url` key of the message. If that’s not possible, no inlining
is done.

## Example usage, in `ggsrc`

	command 2000 "rss2json feed-url | ua-inline | maildir-put"

## Installation

	go build && cp ua-inline /usr/local/bin
