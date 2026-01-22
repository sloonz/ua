package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bmatsuo/lmdb-go/lmdb"
	"gopkg.in/redis.v3"
)

type Cache interface {
	Getset(id, host string) bool
	Exists(id, host string) bool
	Put(id, host string, ts int64) error
	Iterate(fn func(id, host string, ts int64) error) error
	DeleteOlderThan(cutoff int64) (int, error)
	Dump() error
	Close()
}

type cacheOptions struct {
	path         string
	useRedis     bool
	useLmdb      bool
	lmdbPath     string
	lmdbMapSize  int64
	redisOptions redis.Options
}

func NewCache(opts cacheOptions) (Cache, error) {
	if opts.useRedis && opts.useLmdb {
		return nil, errors.New("Can't use both redis and lmdb caches")
	}

	if opts.useRedis {
		return newRedisCache(opts)
	}

	if opts.useLmdb {
		return newLmdbCache(opts)
	}

	return newFileCache(opts.path)
}

func newTimestamp() int64 {
	return time.Now().Unix()
}

func cacheKey(id, host string) string {
	return host + "\x00" + id
}

func encodeTimestampBinary(ts int64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(ts))
	return buf
}

func parseTimestampBinary(b []byte) int64 {
	if len(b) == 8 {
		return int64(binary.BigEndian.Uint64(b))
	}
	return parseTimestamp(string(b))
}

func splitCacheKey(key string) (string, string) {
	parts := strings.SplitN(key, "\x00", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[1], parts[0]
}

func parseTimestamp(s string) int64 {
	if s == "" {
		return 0
	}
	ts, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return ts
}

type fileCache struct {
	data    map[string]int64
	newData map[string]int64
	path    string
}

func newFileCache(path string) (*fileCache, error) {
	c := &fileCache{
		data:    make(map[string]int64),
		newData: make(map[string]int64),
		path:    path,
	}

	cacheFile, err := os.Open(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	} else if os.IsNotExist(err) {
		return c, nil
	}
	defer cacheFile.Close()

	reader := bufio.NewReader(cacheFile)
	for err != io.EOF {
		var key string
		if key, err = reader.ReadString('\n'); err != nil && err != io.EOF {
			return nil, err
		}
		if key != "" {
			key = key[:len(key)-1]
		}
		if key != "" {
			parts := strings.SplitN(key, "\t", 2)
			ts := int64(0)
			if len(parts) > 1 {
				ts = parseTimestamp(parts[1])
			}
			if ts == 0 {
				ts = newTimestamp()
			}
			c.data[parts[0]] = ts
		}
	}

	return c, nil
}

func (c *fileCache) Getset(id, host string) bool {
	key := cacheKey(id, host)
	if _, has := c.data[key]; has {
		return true
	}
	if _, has := c.newData[key]; has {
		return true
	}
	c.newData[key] = newTimestamp()
	return false
}

func (c *fileCache) Exists(id, host string) bool {
	key := cacheKey(id, host)
	if _, has := c.data[key]; has {
		return true
	}
	if _, has := c.newData[key]; has {
		return true
	}
	return false
}

func (c *fileCache) Dump() error {
	cacheFile, err := os.OpenFile(c.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0660)
	if err != nil {
		return err
	}
	defer cacheFile.Close()

	if err = syscall.Flock(int(cacheFile.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}

	writer := bufio.NewWriter(cacheFile)
	for key := range c.newData {
		if _, err = writer.WriteString(key); err != nil {
			return err
		}
		if _, err = writer.WriteString("\t"); err != nil {
			return err
		}
		if _, err = writer.WriteString(strconv.FormatInt(c.newData[key], 10)); err != nil {
			return err
		}
		if _, err = writer.WriteString("\n"); err != nil {
			return err
		}
	}
	if err = writer.Flush(); err != nil {
		return err
	}

	return nil
}

func (c *fileCache) Iterate(fn func(id, host string, ts int64) error) error {
	for key, ts := range c.data {
		id, host := splitCacheKey(key)
		if id == "" {
			continue
		}
		if err := fn(id, host, ts); err != nil {
			return err
		}
	}
	for key, ts := range c.newData {
		id, host := splitCacheKey(key)
		if id == "" {
			continue
		}
		if err := fn(id, host, ts); err != nil {
			return err
		}
	}
	return nil
}

func (c *fileCache) Put(id, host string, ts int64) error {
	key := cacheKey(id, host)
	if ts <= 0 {
		ts = newTimestamp()
	}
	c.newData[key] = ts
	return nil
}

func (c *fileCache) DeleteOlderThan(cutoff int64) (int, error) {
	removed := 0
	kept := make(map[string]int64)

	for key, ts := range c.data {
		if ts < cutoff {
			removed++
			continue
		}
		kept[key] = ts
	}
	for key, ts := range c.newData {
		if ts < cutoff {
			removed++
			continue
		}
		kept[key] = ts
	}

	cacheFile, err := os.OpenFile(c.path, os.O_CREATE|os.O_RDWR, 0660)
	if err != nil {
		return 0, err
	}
	defer cacheFile.Close()

	if err = syscall.Flock(int(cacheFile.Fd()), syscall.LOCK_EX); err != nil {
		return 0, err
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(c.path), "ua-filter-cache-")
	if err != nil {
		return 0, err
	}
	writer := bufio.NewWriter(tmpFile)
	for key, ts := range kept {
		if _, err = writer.WriteString(key); err != nil {
			tmpFile.Close()
			os.Remove(tmpFile.Name())
			return 0, err
		}
		if _, err = writer.WriteString("\t"); err != nil {
			tmpFile.Close()
			os.Remove(tmpFile.Name())
			return 0, err
		}
		if _, err = writer.WriteString(strconv.FormatInt(ts, 10)); err != nil {
			tmpFile.Close()
			os.Remove(tmpFile.Name())
			return 0, err
		}
		if _, err = writer.WriteString("\n"); err != nil {
			tmpFile.Close()
			os.Remove(tmpFile.Name())
			return 0, err
		}
	}
	if err = writer.Flush(); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return 0, err
	}
	if err = tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return 0, err
	}

	if err = os.Rename(tmpFile.Name(), c.path); err != nil {
		os.Remove(tmpFile.Name())
		return 0, err
	}

	c.data = kept
	c.newData = make(map[string]int64)

	return removed, nil
}

func (c *fileCache) Close() {}

type redisCache struct {
	client *redis.Client
	ts     []byte
}

func newRedisCache(opts cacheOptions) (*redisCache, error) {
	return &redisCache{
		client: redis.NewClient(&opts.redisOptions),
		ts:     encodeTimestampBinary(newTimestamp()),
	}, nil
}

func (c *redisCache) Getset(id, host string) bool {
	res := c.client.HExists("ua:"+host, id)
	if res.Err() != nil && res.Err() != redis.Nil {
		log.Fatalf("Error using redis cache: %s", res.Err())
	}

	present := res.Val()

	res = c.client.HSet("ua:"+host, id, string(c.ts))
	if res.Err() != nil && res.Err() != redis.Nil {
		log.Fatalf("Error using redis cache: %s", res.Err())
	}

	return present
}

func (c *redisCache) Exists(id, host string) bool {
	res := c.client.HExists("ua:"+host, id)
	if res.Err() != nil && res.Err() != redis.Nil {
		log.Fatalf("Error using redis cache: %s", res.Err())
	}
	return res.Val()
}

func (c *redisCache) Put(id, host string, ts int64) error {
	if ts <= 0 {
		ts = newTimestamp()
	}
	res := c.client.HSet("ua:"+host, id, string(encodeTimestampBinary(ts)))
	if res.Err() != nil && res.Err() != redis.Nil {
		return res.Err()
	}
	return nil
}

func (c *redisCache) Iterate(fn func(id, host string, ts int64) error) error {
	keys, err := c.client.Keys("ua:*").Result()
	if err != nil && err != redis.Nil {
		return err
	}

	for _, key := range keys {
		host := strings.TrimPrefix(key, "ua:")
		entries, err := c.client.HGetAllMap(key).Result()
		if err != nil && err != redis.Nil {
			return err
		}
		for id, tsStr := range entries {
			if err := fn(id, host, parseTimestampBinary([]byte(tsStr))); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *redisCache) DeleteOlderThan(cutoff int64) (int, error) {
	removed := 0
	keys, err := c.client.Keys("ua:*").Result()
	if err != nil && err != redis.Nil {
		return 0, err
	}

	for _, key := range keys {
		entries, err := c.client.HGetAllMap(key).Result()
		if err != nil && err != redis.Nil {
			return removed, err
		}
		for id, tsStr := range entries {
			if parseTimestampBinary([]byte(tsStr)) < cutoff {
				if err = c.client.HDel(key, id).Err(); err != nil && err != redis.Nil {
					return removed, err
				}
				removed++
			}
		}
	}
	return removed, nil
}

func (c *redisCache) Dump() error {
	return nil
}

func (c *redisCache) Close() {
	if c.client != nil {
		c.client.Close()
	}
}

type lmdbCache struct {
	env *lmdb.Env
	dbi lmdb.DBI
	ts  []byte
}

func newLmdbCache(opts cacheOptions) (*lmdbCache, error) {
	if err := os.MkdirAll(opts.lmdbPath, 0775); err != nil {
		return nil, err
	}

	env, err := lmdb.NewEnv()
	if err != nil {
		return nil, err
	}
	if opts.lmdbMapSize <= 0 {
		opts.lmdbMapSize = 1 << 30
	}
	if err = env.SetMaxDBs(1); err != nil {
		return nil, err
	}
	if err = env.SetMapSize(opts.lmdbMapSize); err != nil {
		return nil, err
	}
	if err = env.Open(opts.lmdbPath, 0, 0664); err != nil {
		return nil, err
	}

	c := &lmdbCache{
		env: env,
		ts:  encodeTimestampBinary(newTimestamp()),
	}
	if err = env.Update(func(txn *lmdb.Txn) error {
		dbi, err := txn.OpenDBI("ua", lmdb.Create)
		if err != nil {
			return err
		}
		c.dbi = dbi
		return nil
	}); err != nil {
		env.Close()
		return nil, err
	}

	return c, nil
}

func (c *lmdbCache) Getset(id, host string) bool {
	key := []byte(cacheKey(id, host))
	present := false
	err := c.env.Update(func(txn *lmdb.Txn) error {
		_, err := txn.Get(c.dbi, key)
		if err == nil {
			present = true
		} else if !lmdb.IsNotFound(err) {
			return err
		}

		return txn.Put(c.dbi, key, c.ts, 0)
	})
	if err != nil {
		log.Fatalf("Error using lmdb cache: %s", err.Error())
	}

	return present
}

func (c *lmdbCache) Exists(id, host string) bool {
	key := []byte(cacheKey(id, host))
	present := false
	err := c.env.View(func(txn *lmdb.Txn) error {
		_, err := txn.Get(c.dbi, key)
		if err == nil {
			present = true
			return nil
		}
		if lmdb.IsNotFound(err) {
			return nil
		}
		return err
	})
	if err != nil {
		log.Fatalf("Error using lmdb cache: %s", err.Error())
	}
	return present
}

func (c *lmdbCache) Put(id, host string, ts int64) error {
	if ts <= 0 {
		ts = newTimestamp()
	}
	key := []byte(cacheKey(id, host))
	val := encodeTimestampBinary(ts)
	return c.env.Update(func(txn *lmdb.Txn) error {
		return txn.Put(c.dbi, key, val, 0)
	})
}

func (c *lmdbCache) Iterate(fn func(id, host string, ts int64) error) error {
	return c.env.View(func(txn *lmdb.Txn) error {
		cur, err := txn.OpenCursor(c.dbi)
		if err != nil {
			return err
		}
		defer cur.Close()

		for {
			key, val, err := cur.Get(nil, nil, lmdb.Next)
			if lmdb.IsNotFound(err) {
				break
			}
			if err != nil {
				return err
			}
			id, host := splitCacheKey(string(key))
			if id == "" {
				continue
			}
			if err := fn(id, host, parseTimestampBinary(val)); err != nil {
				return err
			}
		}
		return nil
	})
}

func (c *lmdbCache) DeleteOlderThan(cutoff int64) (int, error) {
	removed := 0
	err := c.env.Update(func(txn *lmdb.Txn) error {
		cur, err := txn.OpenCursor(c.dbi)
		if err != nil {
			return err
		}
		defer cur.Close()

		for {
			_, val, err := cur.Get(nil, nil, lmdb.Next)
			if lmdb.IsNotFound(err) {
				break
			}
			if err != nil {
				return err
			}
			if parseTimestampBinary(val) < cutoff {
				if err = cur.Del(0); err != nil {
					return err
				}
				removed++
			}
		}
		return nil
	})
	return removed, err
}

func (c *lmdbCache) Dump() error {
	return nil
}

func (c *lmdbCache) Close() {
	if c.env != nil {
		c.env.Close()
	}
}
