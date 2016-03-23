# ua-proxify -- Transform external URLs in a message

This is a simple filter intended to be used before `maildir-put`. It
changes the URL of external resources (CSS, images).

If the body contains relative references, it tries to resolve them using
the `url` key of the message. If that’s not possible, no change
is done.

## Example usage, in `ggsrc`

`get.php` is a simple example script provided with `ua-proxify`. It can be used
that way:

	command 2000 "rss2json feed-url | \
		ua-proxify "http://example.com/get?url={{.URL|urlquery}}&sig={{.URL|HMAC \"$HMAC_KEY\"}}" | \
		maildir-put"

`$HMAC_KEY` can be generated with `openssl rand -base64 32` and must be set in
the top of `get.php`.

## Installation

	go build && cp ua-proxify /usr/local/bin
