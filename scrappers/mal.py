import scrapy
import json
import re

class Animes(scrapy.Spider):
    name = "mal-animes"
    start_urls = ["http://myanimelist.net/anime/season"]

    def parse(self, response):
        for item in response.css('.seasonal-anime'):
            title = item.css('.link-title')[0]
            genres = item.css('.genres')[0]
            desc = item.css('.synopsis')[0]
            link = title.css('::attr("href")')[0].extract()
            try:
                img = item.css('.image img::attr("src")')[0].extract()
            except:
                img = item.css('.image img::attr("data-src")')[0].extract()
            img_tag = '<img src="%s" />' % img

            print((json.dumps({
                'url': link,
                'id': link,
                'title': ' '.join(title.css('::text').extract()),
                'body': '%s %s %s %s' % (title.extract(), img_tag, desc.extract(), genres.extract()),
                'host': 'myanimelist.net'
            })))
