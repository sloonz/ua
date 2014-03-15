# The Universal Aggregator

This is a set of tools to aggregate all your information into your
maildir. Each tool can be used separately ; you can find a more complete
description in their respective folder.

* `ggs` is a software which runs commands periodically
* `maildir-put` reads a set of messages from its standard input and puts
them in a maildir
* `rss2json` transforms any RSS/Atom feed into a set of messages that
`maildir-put` can process
* You can write your own producers for maildir-put ; an example for the
[mangareader](http://mangareader.net) service is provided.
* You can also put filters, like `ua-inline`

## Usage

	ggs [path-to-configuration-file]

## Dependencies

* Go
* libxml
* For additional scrappers: python 3, aiohttp and pyquery

## Installation

	make && sudo make install

## Configuration

See the `ggs` documentation for more information. Here is an sample
configuration file, which puts some feeds into `Fun` and `Geek` folders,
some new chapters notification from mangareader into `Entertainment`,
and my Github personal feed into inbox:

	default_timeout=30

	rss() {
		command 2000 "rss2json \"$1\" | ua-inline | maildir-put -root $HOME/Maildir-feeds -folder \"$2\""
	}

	mangareader() {
		command 2000 "mangareader2json http://mangareader.net/$1 | "\
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
