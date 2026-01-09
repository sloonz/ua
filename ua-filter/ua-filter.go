package main

import (
	"encoding/json"
	"errors"
	"flag"
	"github.com/bmatsuo/lmdb-go/lmdb"
	"io"
	"log"
	"os"
	"time"
)

type Message map[string]interface{}

var hostname string
var cache Cache

func getStringField(msg Message, key string) string {
	if val, ok := msg[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func normalizeMessage(msg Message) error {
	if getStringField(msg, "body") == "" || getStringField(msg, "title") == "" {
		return errors.New("Missing mandatory field")
	}

	host := getStringField(msg, "host")
	if host == "" {
		host = hostname
		msg["host"] = host
	}

	if getStringField(msg, "authorEmail") == "" {
		msg["authorEmail"] = "noreply@" + host
	}

	if getStringField(msg, "date") == "" {
		msg["date"] = time.Now().UTC().Format(time.RFC1123Z)
	}

	if getStringField(msg, "id") == "" {
		url := getStringField(msg, "url")
		if url != "" {
			msg["id"] = url
		}
	}

	return nil
}

func shouldDrop(msg Message) bool {
	id := getStringField(msg, "id")
	if id == "" {
		return false
	}

	host := getStringField(msg, "host")
	return cache.Getset(id, host)
}

func runMigration(srcOpts cacheOptions, dstOpts cacheOptions) {
	var err error

	src, err := NewCache(srcOpts)
	if err != nil {
		log.Fatalf("Can't open source cache: %s", err.Error())
	}
	defer src.Close()

	dst, err := NewCache(dstOpts)
	if err != nil {
		log.Fatalf("Can't open destination cache: %s", err.Error())
	}
	defer dst.Close()

	log.Printf("Starting cache migration")
	processed := 0
	lastLog := time.Now()
	if dstLmdb, ok := dst.(*lmdbCache); ok {
		err = dstLmdb.env.Update(func(txn *lmdb.Txn) error {
			return src.Iterate(func(id, host string, ts int64) error {
				processed++
				if processed == 1 || time.Since(lastLog) > 5*time.Second {
					log.Printf("Migrated %d entries (latest id=%q host=%q)", processed, id, host)
					lastLog = time.Now()
				}
				if ts <= 0 {
					ts = time.Now().Unix()
				}
				key := []byte(cacheKey(id, host))
				val := encodeTimestampBinary(ts)
				return txn.Put(dstLmdb.dbi, key, val, 0)
			})
		})
	} else {
		err = src.Iterate(func(id, host string, ts int64) error {
			processed++
			if processed == 1 || time.Since(lastLog) > 5*time.Second {
				log.Printf("Migrated %d entries (latest id=%q host=%q)", processed, id, host)
				lastLog = time.Now()
			}
			return dst.Put(id, host, ts)
		})
	}
	if err != nil {
		log.Fatalf("Can't migrate cache: %s", err.Error())
	}
	log.Printf("Finished cache migration: %d entries", processed)

	if err = dst.Dump(); err != nil {
		log.Fatalf("Can't dump destination cache: %s", err.Error())
	}
}

func runGC(opts cacheOptions, ttl string) {
	cache, err := NewCache(opts)
	if err != nil {
		log.Fatalf("Can't open cache: %s", err.Error())
	}
	defer cache.Close()

	duration, err := time.ParseDuration(ttl)
	if err != nil {
		log.Fatalf("Can't parse gc duration: %s", err.Error())
	}
	cutoff := time.Now().Add(-duration).Unix()
	removed, err := cache.DeleteOlderThan(cutoff)
	if err != nil {
		log.Fatalf("Can't garbage-collect cache: %s", err.Error())
	}
	if err = cache.Dump(); err != nil {
		log.Fatalf("Can't dump cache: %s", err.Error())
	}
	log.Printf("Garbage-collected %d entries", removed)
}

func main() {
	var err error
	var opts cacheOptions
	var migrateTo string
	var gcTTL string
	var migrateCachePath string
	var migrateRedisAddr string
	var migrateRedisDB int64
	var migrateRedisPassword string
	var migrateLmdbPath string
	var migrateLmdbMapSize int64

	flag.StringVar(&opts.path, "cache", os.ExpandEnv("$HOME/.cache/maildir-put.cache"),
		"path to store message-ids to drop duplicate messages")
	flag.BoolVar(&opts.useRedis, "redis", false, "use redis for cache storage")
	flag.StringVar(&opts.redisOptions.Addr, "redis-addr", "127.0.0.1:6379", "redis address")
	flag.Int64Var(&opts.redisOptions.DB, "redis-db", 0, "redis base")
	flag.StringVar(&opts.redisOptions.Password, "redis-password", "", "redis password")
	flag.BoolVar(&opts.useLmdb, "lmdb", false, "use lmdb for cache storage")
	flag.StringVar(&opts.lmdbPath, "lmdb-path", os.ExpandEnv("$HOME/.cache/ua-filter.lmdb"), "lmdb database path")
	flag.Int64Var(&opts.lmdbMapSize, "lmdb-map-size", 1<<30, "lmdb map size in bytes")
	flag.StringVar(&migrateTo, "migrate-to", "", "migrate cache entries to backend (redis|lmdb|file)")
	flag.StringVar(&gcTTL, "gc", "", "garbage-collect entries older than the given duration (e.g. 168h)")
	flag.StringVar(&migrateCachePath, "migrate-cache", "", "destination cache file for migration")
	flag.StringVar(&migrateRedisAddr, "migrate-redis-addr", "", "destination redis address for migration")
	flag.Int64Var(&migrateRedisDB, "migrate-redis-db", -1, "destination redis base for migration")
	flag.StringVar(&migrateRedisPassword, "migrate-redis-password", "", "destination redis password for migration")
	flag.StringVar(&migrateLmdbPath, "migrate-lmdb-path", "", "destination lmdb path for migration")
	flag.Int64Var(&migrateLmdbMapSize, "migrate-lmdb-map-size", 0, "destination lmdb map size in bytes")

	if flag.Parse(); !flag.Parsed() {
		flag.PrintDefaults()
		os.Exit(1)
	}

	if migrateTo != "" && gcTTL != "" {
		log.Fatal("Can't use migrate and gc together")
	}

	if migrateTo != "" {
		dst := cacheOptions{
			path:         opts.path,
			lmdbPath:     opts.lmdbPath,
			lmdbMapSize:  opts.lmdbMapSize,
			redisOptions: opts.redisOptions,
		}

		switch migrateTo {
		case "redis":
			dst.useRedis = true
		case "lmdb":
			dst.useLmdb = true
		case "file":
		default:
			log.Fatal("Invalid migrate-to value")
		}

		if migrateCachePath != "" {
			dst.path = migrateCachePath
		}
		if migrateRedisAddr != "" {
			dst.redisOptions.Addr = migrateRedisAddr
		}
		if migrateRedisDB >= 0 {
			dst.redisOptions.DB = migrateRedisDB
		}
		if migrateRedisPassword != "" {
			dst.redisOptions.Password = migrateRedisPassword
		}
		if migrateLmdbPath != "" {
			dst.lmdbPath = migrateLmdbPath
		}
		if migrateLmdbMapSize > 0 {
			dst.lmdbMapSize = migrateLmdbMapSize
		}

		runMigration(opts, dst)
		return
	}

	if gcTTL != "" {
		runGC(opts, gcTTL)
		return
	}

	if cache, err = NewCache(opts); err != nil {
		log.Fatalf("Can't open cache: %s", err.Error())
	}
	defer cache.Close()

	if hostname, err = os.Hostname(); err != nil {
		log.Fatalf("Can't get hostname: %s", err.Error())
	}

	dec := json.NewDecoder(os.Stdin)
	enc := json.NewEncoder(os.Stdout)
	for {
		msg := Message{}
		err = dec.Decode(&msg)
		if err == nil {
			err = normalizeMessage(msg)
		}
		if err == nil && shouldDrop(msg) {
			err = nil
			continue
		}
		if err == nil {
			err = enc.Encode(msg)
		}

		if err == io.EOF {
			break
		} else if err != nil {
			log.Printf("Cannot process input message: %s", err.Error())
		}
	}

	if err = cache.Dump(); err != nil {
		log.Printf("warning: can't dump cache: %s", err.Error())
	}
}
