# ua-filter

`ua-filter` is a filter intended to be used before `maildir-put`. It
normalizes message fields and drops duplicates using a cache.

## Usage

	message-producer | ua-filter [arguments] | maildir-put

Available arguments:

* `-cache`: path to a cache file used to store message IDs for duplicate
  detection
* `-redis`: specify this flag to use redis for message IDs cache
* `-redis-db`, `-redis-addr`, `-redis-password`: redis connection settings
* `-lmdb`: specify this flag to use lmdb for message IDs cache
* `-lmdb-path`: path to the lmdb database
* `-lmdb-map-size`: lmdb map size in bytes
* `-mode`: message handling mode: `filter` (default), `check`, or `mark`
* `-migrate-to`: migrate cache entries to backend (`redis`, `lmdb`, `file`)
* `-migrate-cache`: destination cache file for migration
* `-migrate-redis-addr`, `-migrate-redis-db`, `-migrate-redis-password`:
  destination redis connection settings for migration
* `-migrate-lmdb-path`, `-migrate-lmdb-map-size`: destination lmdb settings
  for migration
* `-gc`: garbage-collect entries older than the given duration (for example
  `168h`)

## Installation

	go build && cp ua-filter /usr/local/bin

## Input/output format

`ua-filter` reads a stream of JSON dictionaries (not a list of
dictionaries) and outputs the same format.

It ensures that:

* *host* defaults to the machine hostname
* *authorEmail* defaults to `noreply@<host>`
* *date* defaults to the current time (RFC 2822)

Messages missing mandatory fields (*body* or *title*) are dropped.

## Modes

`ua-filter` supports three modes (set with `-mode`):

* `filter`: drop duplicates and mark them in the cache (default)
* `check`: drop duplicates without marking them in the cache
* `mark`: mark messages in the cache without dropping them

`check` is useful before a sender that may fail. In that case you can
mark only after a successful send:

	rss2json url | ua-filter -mode check | ua-send | ua-filter -mode mark

## Migration

To migrate from the current cache backend to another one, use
`-migrate-to` and set destination options as needed:

	ua-filter -cache ~/.cache/ua.cache -migrate-to lmdb -migrate-lmdb-path ~/.cache/ua.lmdb
	ua-filter -redis -redis-addr 127.0.0.1:6379 -migrate-to file -migrate-cache ~/.cache/ua.cache

## Garbage collection

To remove entries older than a given duration:

	ua-filter -lmdb -lmdb-path ~/.cache/ua.lmdb -gc 168h
