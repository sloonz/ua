# The Universal Aggregator

This is a set of tools to aggregate all your information into your
maildir. Each tool can be used separately ; you can find a more complete
description in their respective folder.

* `ggs` is a software which runs commands periodically
* `ua-filter` normalizes messages and drops duplicates
* `maildir-put` reads a set of messages from its standard input and puts
them in a maildir
* `rss2json` transforms any RSS/Atom feed into a set of messages that
`maildir-put` can process
* `ua-send` sends messages in the same format as `maildir-put` to a SMTP or
LMTP server
* You can write your own producers (scrapers) for maildir-put ; some are
already provided in the `scrapers/` directory.
* You can also put filters, like `ua-filter`, `ua-inline` or `ua-proxify`.

## Usage

	ggs [path-to-configuration-file]

## Dependencies

* Go
* libxml
* [jq](https://stedolan.github.io/jq/)
* For additional scrapers: scrapy, python 3 and nodejs

## Installation

	make && sudo make install

## Configuration

See the `ggs` documentation for more information. Here is an sample
configuration file, which puts some feeds into `Fun` and `Geek` folders,
some new chapters notification from mangareader into `Entertainment`,
and my Github personal feed into inbox:

	default_timeout=30

	rss() {
		command 2000 "rss2json \"$1\" | ua-inline | ua-filter | maildir-put -root $HOME/Maildir-feeds -folder \"$2\""
	}

	mangareader() {
		command 2000 "ua-scraper-mangareader -a name=$1 | ua-filter | "\
			"maildir-put -root $HOME/Maildir-feeds -folder Entertainment"
	}

	rss http://xkcd.com/atom.xml Fun
	rss http://feeds.feedburner.com/smbc-comics/PvLb Fun
	rss http://syndication.thedailywtf.com/TheDailyWtf Fun

	rss http://www.reddit.com/r/science/top/.rss Geek
	rss http://www.phoronix.com/rss.php Geek
	
	mangareader naruto
	mangareader bleach
	mangareader gantz

	rss https://github.com/sloonz.private.atom?token=HIDDEN ""

## Weboob compatibility

You can use [weboob](http://weboob.org/) modules used by
[boobmsg](http://weboob.org/applications/boobmsg) to generate
messages. Configure the modules using `boobmsg`, and use `weboobmsg2json
[module-name]` to generate messages. `[module-name]` can be found in
`~/.config/weboob/backends`.
