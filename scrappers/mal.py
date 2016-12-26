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
            img = re.findall(r'url\s*\(\s*([^\s)]+)', item.css('.image::attr("style")')[0].extract())[0]
            img_tag = u'<img src="%s" />' % img

            print(json.dumps({
                'url': link,
                'id': link,
                'title': u' '.join(title.css('::text').extract()),
                'body': u'%s %s %s %s' % (title.extract(), img_tag, desc.extract(), genres.extract()),
                'host': 'myanimelist.net'
            }))
