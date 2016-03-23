# Usage: scrapy edxcourses.py | ... | maildir-putt
# -*- encoding: utf-8 -*-

import json
import urlparse
import time
import scrapy
import email.utils

class EdxCourse:
    def __init__(self, data):
        self.data = data

    def parse_description(self, response):
        def do_parse(response):
            self.data["body"] += json.loads(response.body)["description"]
            print json.dumps(self.data)

        yield scrapy.Request("https://www.edx.org/api/catalog/v2/courses/%s" % response.css('main ::attr(data-course-id)').extract()[0],
                callback=do_parse)


class EdxCourses(scrapy.Spider):
    name = "edx-courses"
    start_urls = ["https://www.edx.org/search/api/all"]

    def parse(self, response):
        for course in json.loads(response.body):
            if not u"English" in course["languages"]:
                continue
            if u"profed" in course["types"]:
                continue

            body = u'<p><a href="%s">%s — %s</a> (%s)</p>' % (course["url"], course["code"], course["l"], course["start"])
            body += u'<p><a href="%s"><img src="%s"></a></p>' % (course["url"], urlparse.urljoin(self.start_urls[0], course["image"]["src"]))

            course_item = EdxCourse({
                "url": course["url"],
                "title": u"%s %s" % (u" ".join("[%s]"%_ for _ in course["schools"]), course["l"]),
                "id": u"%s.%s.edx" % (course["l"], u".".join(course["schools"])),
                "date": email.utils.formatdate(course["start_time"]),
                "body": body
            })
            
            yield scrapy.Request(course["url"], callback=course_item.parse_description)
