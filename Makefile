PREFIX=/usr/local
DESTDIR=

BINDIR=$(DESTDIR)$(PREFIX)/bin
DOCDIR=$(DESTDIR)$(PREFIX)/share/doc/ua
MANDIR=$(DESTDIR)$(PREFIX)/share/man

GODIRS=ggs rss2json maildir-put ua-inline ua-proxify
SCRAPERS=edxcourses lyon-bm-bd mal mangareader yggtorrent torrent9 bookys

export GOPATH ?= $(PWD)/tmp-go

.PHONY: all clean doc

all: ggs/ggs rss2json/rss2json maildir-put/maildir-put ua-inline/ua-inline ua-proxify/ua-proxify \
	scrapers/ua-scraper-torrent9

doc:
	test -d doc || mkdir doc
	test -f doc/ua.md || ln -s ../README.md doc/ua.md
	test -f doc/ua-scrapers.md || ln -s ../scrapers/README.md doc/ua-scrapers.md
	for d in $(GODIRS) ; do test -f doc/$$d.md || ln -s ../$$d/README.md doc/$$d.md ; done
	cd doc ; for f in *.md ; do ronn $$f ; done

ggs/ggs: ggs/ggs.go $(GOPATH)
	cd ggs; go get -d && go build

rss2json/rss2json: rss2json/rss2json.go $(GOPATH)
	cd rss2json; go get -d && go build

maildir-put/maildir-put: maildir-put/maildir-put.go maildir-put/cache.go $(GOPATH)
	cd maildir-put; go get -d && go build

ua-inline/ua-inline: ua-inline/ua-inline.go $(GOPATH)
	cd ua-inline; go get -d && go build

ua-proxify/ua-proxify: ua-proxify/ua-proxify.go $(GOPATH)
	cd ua-proxify; go get -d && go build

$(GOPATH):
	mkdir $(GOPATH)
	mkdir $(GOPATH)/bin
	mkdir $(GOPATH)/src
	mkdir $(GOPATH)/pkg

install: all
	install -d $(BINDIR)
	for f in $(GODIRS) ; do install $$f/$$f $(BINDIR)/ ; done
	for s in $(SCRAPERS) ; do install scrapers/ua-scraper-$$s $(BINDIR)/ ; done
	install weboobmsg2json/weboobmsg2json $(BINDIR)/
	
	install -d $(DOCDIR)
	install -d $(MANDIR)/man1/
	install ggsrc.example $(DOCDIR)
	for f in doc/*.md doc/*.html ; do install $$f $(DOCDIR)/ ; done
	for f in $(GODIRS) ; do gzip < doc/$$f > $(MANDIR)/man1/$$f.1.gz ; done

clean:
	for f in $(GODIRS) ; do rm -f $$f/$$f ; done
	rm -rf tmp-go scapers/node_modules scrapers/ua-scraper-torrent9
