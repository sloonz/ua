const cloudscraper = require('cloudscraper');
const cheerio = require('cheerio');
const process = require('process');
const urljoin = require('url').resolve;
const childProcess = require('child_process');
const url = require('url');

const BASE_URL = 'http://www.torrent9.biz/';

let filters = (process.argv[2] || 'film series musique ebook jeux-pc jeux-console logiciels').split(/\s+/);

cloudscraper.get(BASE_URL, (err, resp, body) => {
	if(err) {
		throw err;
	}

	let $ = cheerio.load(body);
	for(let filter of filters) {
		$(`.${filter}-table a`).each((_, itemLink) => {
			let itemUrl = url.resolve(BASE_URL, itemLink.attribs.href);
			cloudscraper.get(itemUrl, (err, resp, body) => {
				if(err) {
					throw err;
				}

				let $ = cheerio.load(body);
				let cover = $('.movie-img img');
				let desc = $('.movie-information');
				let title = $('h5').text().replace(/(^\s+)|(\s+$)/g, '').replace(/\s+/g, ' ');
				cover.attr('style', 'max-height:250px;width:100%');
				$('ul', desc).remove();

				console.log(JSON.stringify({
					title,
					id: itemUrl,
					url: itemUrl,
					host: 'torrent9.biz',
					body: `<span style="float:left;margin:3px;">${$.html(cover)}</span> ${desc.html()} <p><a href="${itemUrl}">View post</a></p>`
				}));
			});
		});
	}
});
