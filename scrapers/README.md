This contains additional scrapers. You can take those as examples to
write your own.

# ua-scraper-exdcourses

List all courses on [EdX](https://www.edx.org/).

# ua-scraper-lyon-bm-bd

List new comics on [Lyon public library](https://www.bm-lyon.fr/).

# ua-scraper-mal

List season animes from [myanimelist](https://myanimelist.net/anime/season).

# ua-scraper-mangareader

List latest chapters for a given manga on [mangareader](http://www.mangareader.net/).

Usage: `ua-scraper-mangareader -a name=[manga-title]`. `[manga-title]`
is the path of the manga on mangareader, for example `natsume-yuujinchou`
for http://www.mangareader.net/natsume-yuujinchou.

# ua-scraper-torrent9

List latest torrents on [torrent9](http://www.torrent9.cc/).

Usage:

* All categories: `ua-scraper-torrent9`
* Specific categories: `ua-scraper-torrent9 "category1 category2..."`

Categories references the anchor in the URL (for example `ebook` for
http://www.torrent9.cc/#ebook).

# ua-scraper-yggtorrent

List lastest torrents on [yggtorrent](https://yggtorrent.com/).

Usage:

* All categories: `ua-scraper-yggtorrent`
* Specific category: `ua-scraper-yggtorrent [url]`.
