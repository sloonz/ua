import scrapy
import json
import urllib.parse

class BMLyonBD(scrapy.Spider):
    name = "bmlyon-bd"
    start_urls = ["http://catalogue.bm-lyon.fr/alswww2.dll/APS_ZONES?fn=Search&q=%23%23Nouveautes&Style=Portal3&SubStyle=&Lang=FRE&ResponseEncoding=utf-8"]

    def parse(self, response):
        for link in response.css("a.FacetLinkA::attr('href')").extract():
            if "bande_dessinees" in link:
                yield scrapy.Request(urllib.parse.urljoin(response.url, link), callback=self.parse_page)

    def parse_book(self, response, desc):
        body = response.css("td.DetailDataCell > table > tr > td > table").extract()[0]
        link = urllib.parse.urljoin(response.url, response.css("a[rel='bookmark']::attr('href')").extract()[0])
        img = response.css("img.LargeBookCover::attr('src')").extract()
        if img:
            img = urllib.parse.urljoin(response.url, img[0])
            body = '<div><img src="%s"></div><div>%s</div>' % (img, body)

        print(json.dumps({
            'title': desc,
            'body': body,
            'url': link,
            'id': link,
            'host': 'bm-lyon.fr'
            }))

    def parse_page(self, response):
        def fetch_book(link, desc):
            return scrapy.Request(urllib.parse.urljoin(response.url, link),
                    callback=lambda r: self.parse_book(r, desc),
                    dont_filter=True)

        for item in response.css("#BrowseList a.SummaryFieldLink"):
            link = item.css('::attr("href")').extract()[0]
            desc = " ".join(item.css("::text").extract())
            yield fetch_book(link, desc)

        for item in response.css("a.pageNavLink::attr('href')").extract():
            if "PageDown" in item:
                yield scrapy.Request(urllib.parse.urljoin(response.url, item), callback=self.parse_page, dont_filter=True)
                break

