import scrapy
import json
import urllib.parse

# Usage:
# All categories:
#   scrapy runspider yggtorrent.py
# Specific category:
#   scrapy runspider yggtorrent.py -a url=https://yggtorrent.com/torrents/filmvideo/2183-film

class YggTorrent(scrapy.Spider):
    name = "yggtorrent"

    def __init__(self, *args, **kwargs):
        super(YggTorrent, self).__init__(*args, **kwargs)
        self.start_urls = [kwargs.get('url', 'https://yggtorrent.com/')]

    def parse_item(self, response):
        print((json.dumps({
            'title': u" ".join(response.css('.panel-title')[0].css('::text').extract()),
            'body': '%s<p><a href="View torrent">%s</a></p>' % (response.css('#description').extract()[0], response.url),
            'id': response.url,
            'host': 'yggtorrent.com',
            'url': response.url
            })))

    def parse(self, response):
        for item in response.css('a.torrent-name'):
            url = urllib.parse.urljoin(response.url, item.css('::attr("href")').extract()[0])
            yield scrapy.Request(url, self.parse_item)
