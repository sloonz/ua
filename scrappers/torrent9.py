import scrapy
import json
import re
import urlparse

# Usage: scrapy runspider torrent9.py [-a filters="film series"]

class Torrent9(scrapy.Spider):
    name = "torrent9"
    start_urls = ["http://www.torrent9.biz/"]

    def __init__(self, *args, **kwargs):
        self.filters = kwargs.get('filters', 'film series musique ebook jeux-pc jeux-console logiciels').split()

    def parse_item(self, response):
        cover = re.sub(r'<img', '<img style="max-height:250px;width:100%"', response.css('.movie-img img').extract()[0])
        body = response.css('.movie-information').extract()[0]
        body = re.sub(ur'<ul[^>]*>.*?</ul>', u'', body, flags=re.S)
        print(json.dumps({
            'title':  u' '.join(response.css('h5 ::text').extract()).strip(),
            'body': u'<span style="float:left;margin:3px;">%s</span> %s <p><a href="%s">View post</a></p>' % (cover, body, response.url),
            'id': response.url,
            'host': 'torrent9.biz',
            'url': response.url
            }))

    def parse(self, response):
        for filter in self.filters:
            for item in response.css('.%s-table a' % filter):
                url = urlparse.urljoin(response.url, item.css('::attr("href")').extract()[0])
                yield scrapy.Request(url, callback=self.parse_item)
