import scrapy
import json
import urlparse

# Usage: scrapy runspider t411.py -a cat=402
#  (change category id)

class T411(scrapy.Spider):
    name = "t411"

    def __init__(self, *args, **kwargs):
        super(T411, self).__init__(*args, **kwargs)
        self.start_urls = ["http://www.t411.li/torrents/search/?cat=%s&order=added&type=desc" % kwargs['cat']]

    def parse_item(self, response, title):
        print(json.dumps({
            'title': title,
            'body': response.css('.torrentDetails .description').extract()[0],
            'id': response.url,
            'host': 't411.ch',
            'url': response.url
            }))

    def fetch_item(self, url, title):
        return scrapy.Request(url, callback=lambda resp: self.parse_item(resp, title))

    def parse(self, response):
        for item in response.css('table.results > tbody > tr > td:nth-child(2) > a:nth-child(1)'):
            url = urlparse.urljoin(response.url, item.css('::attr("href")').extract()[0])
            title = u" ".join(item.css('::text').extract())
            yield self.fetch_item(url, title)
