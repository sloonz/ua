import asyncio
import base64
import aiohttp
import pyquery
import urllib.parse
import http.cookiejar
import os
import sys
import re
import hashlib

urljoin = urllib.parse.urljoin

cookie_jar = None

def wait(coroutines):
    coroutines = list(coroutines)
    if not coroutines:
        f = asyncio.futures.Future()
        f.set_result([])
        return (yield from f)
    else:
        return (yield from asyncio.wait(coroutines))[0]

def debug(msg):
    print(msg, file = sys.stderr)

def open_cookies(cookie_file = None):
    global cookie_jar
    if cookie_file is None:
        cookie_jar = http.cookiejar.CookieJar()
    else:
        cookie_jar = http.cookiejar.MozillaCookieJar(os.path.expanduser(cookie_file))
        load_cookies()

def load_cookies():
    if not isinstance(cookie_jar, http.cookiejar.FileCookieJar):
        return
    if os.path.exists(cookie_jar.filename) and os.stat(cookie_jar.filename).st_size > 0:
        cookie_jar.load()

def save_cookies():
    if not isinstance(cookie_jar, http.cookiejar.FileCookieJar):
        return
    if not os.path.exists(cookie_jar.filename):
        parent_dir = os.path.dirname(cookie_jar.filename)
        if not os.path.exists(parent_dir):
            os.makedirs(parent_dir)
        open(cookie_jar.filename, "w+").close()
    cookie_jar.save()

def url_quote_unicode(url):
    res = b""
    for c in url.encode('utf-8', 'surrogateescape'):
        if c > 127:
            res += ("%%%02x" % c).encode('utf-8').upper()
        else:
            res += bytes([c])
    return res.decode('utf-8')

class PyQueryWrapper(pyquery.PyQuery):
    def __init__(self, *args, **kwargs):
        self._response = kwargs.pop('response', None)
        base_url = kwargs.pop("base_url", None)
        pyquery.PyQuery.__init__(self, *args, **kwargs)
        if base_url and not self._base_url:
            self._base_url = base_url

    def url(self, attr = None, base_url = None):
        """
        Get the absolute url of this element (for a href, img src...)
        """
        if base_url is None:
            base_url = self.base_url
        if attr is None:
            for attr in ("href", "src", "action"):
                if self.attr(attr):
                    return url_quote_unicode(urllib.parse.urljoin(base_url, self.attr(attr)))
        return url_quote_unicode(urllib.parse.urljoin(base_url, self.attr(attr)))

    def pq(self):
        """
        Equilavent to map(PyQueryWrapper, iter(self))
        Returns PyQuery-like objects for all elements contained in this
        object
        """
        for el in self:
            yield self.__class__(el, parent = self)

    def fetch(self, *args, **kwargs):
        """
        Simple wrapper around fetch() that sets the referer according
        to the request used to retrieve current document
        """
        if "headers" not in kwargs:
            kwargs["headers"] = {}
        kwargs["headers"]["Referer"] = self.base_url
        return fetch(*args, **kwargs)

    @property
    def response(self):
        """
        HTTP response object for the request used to retrive this page
        """
        return self._response or (self.parent and self.parent.response or None)

def _simple_cookie_to_cookie(name, sc, host):
    expires = sc["expires"] and http.cookiejar.http2time(sc["expires"]) or 0
    return http.cookiejar.Cookie(
           None, name, sc.value, None, False,
           sc["domain"].lstrip(".") or host, bool(sc["domain"]), sc["domain"].startswith("."),
           sc["path"] or "/", bool(sc["path"]),
           bool(sc["secure"]), expires, False, None, None, {})

def fetch(url, method = "GET", **kwargs):
    # TODO: don't assume that body is UTF-8
    raw = kwargs.pop('raw_response', False)

    if cookie_jar:
        if not "cookies" in kwargs:
            kwargs["cookies"] = {}
        for c in cookie_jar:
            kwargs["cookies"][c.name] = c.value

    # Handle redirect since aiohttp can be buggy there
    # TODO: report the bug to aiohttp
    allow_redirects = kwargs.pop('allow_redirects', True)
    max_redirects = kwargs.pop('max_redirects', 10)
    kwargs["allow_redirects"] = False
    resp = yield from aiohttp.request(method, url, **kwargs)
    while resp.status // 100 == 3 and allow_redirects and max_redirects > 0:
        headers = dict(resp.message.headers)
        r_url = url_quote_unicode(headers.get('LOCATION') or headers.get('URI'))
        url = urljoin(url, r_url)
        resp = yield from aiohttp.request(method, re.sub(r"#.*", "", url_quote_unicode(url)), **kwargs)
        max_redirects -= 1
        if max_redirects == 0:
            raise Exception("max redirections happened")

    if cookie_jar is not None:
        # TODO: lock this file
        load_cookies()
        for name, cookie in resp.cookies.items():
            cookie_jar.set_cookie(_simple_cookie_to_cookie(name, cookie, resp.host))
        save_cookies()

    if raw:
        return resp
    else:
        body = yield from resp.read()
        return PyQueryWrapper(body.decode("utf-8"), response = resp, base_url = url)

def main(entrypoint):
    asyncio.get_event_loop().run_until_complete(entrypoint)
