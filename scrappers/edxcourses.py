# Usage: scrapy edxcourses.py | ... | maildir-putt
# -*- encoding: utf-8 -*-

import json
import urlparse
import time
import scrapy

class EdxCourses(scrapy.Spider):
    name = "edx-courses"
    start_urls = ["https://www.edx.org/search/api/all"]

    def parse(self, response):
        for course in json.loads(response.body):
            if not u"English" in course["languages"]:
                continue
            if u"profed" in course["types"]:
                continue
            
            yield scrapy.Request(course["url"],
                    callback=lambda r:self.parse_course(r, course))
    
    def parse_course(self, response, summary):
        def do_parse(response):
            body = u"<p>%s — %s (%s)</p>" % (summary["code"], summary["l"], summary["start"])
            body += u'<p><img src="%s"></p>' % (urlparse.urljoin(self.start_urls[0], summary["image"]["src"]),)
            body += json.loads(response.body)["description"]

            print json.dumps({
                "url": summary["url"],
                "title": u"%s %s" % (u" ".join("[%s]"%_ for _ in summary["schools"]), summary["l"]),
                "body": body
            })

        yield scrapy.Request("https://www.edx.org/api/catalog/v2/courses/%s" % response.css('main ::attr(data-course-id)').extract()[0],
                callback=do_parse)
