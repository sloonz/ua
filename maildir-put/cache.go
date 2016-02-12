package main

import (
	"bufio"
	"encoding/binary"
	"gopkg.in/redis.v3"
	"io"
	"log"
	"os"
	"syscall"
	"time"
)

type Cache struct {
	data         map[string]bool
	newData      map[string]bool
	ts           []byte
	path         string
	useRedis     bool
	redisClient  *redis.Client
	redisOptions redis.Options
}

func (c *Cache) OpenCache() (err error) {
	var key string

	c.data = make(map[string]bool)
	c.newData = make(map[string]bool)
	c.ts = make([]byte, 8)

	binary.PutVarint(c.ts, time.Now().Unix())

	if c.useRedis {
		c.redisClient = redis.NewClient(&c.redisOptions)
	}

	cacheFile, err := os.Open(c.path)
	if err != nil && !os.IsNotExist(err) {
		return err
	} else if os.IsNotExist(err) {
		return nil
	}

	reader := bufio.NewReader(cacheFile)
	for err != io.EOF {
		if key, err = reader.ReadString('\n'); err != nil && err != io.EOF {
			return err
		}
		if key != "" && key != "" {
			key = key[:len(key)-1]
			c.data[key] = true
			if c.useRedis {
				c.Getset(key)
			}
		}
	}

	if c.useRedis {
		os.Remove(c.path)
	}

	return nil
}

func (c *Cache) Getset(key string) bool {
	if c.useRedis {
		res := c.redisClient.GetSet(key, c.ts)
		if res.Err() != nil && res.Err() != redis.Nil {
			log.Fatalf("Error using redis cache: %s", res.Err())
		}
		return res.Err() != redis.Nil
	} else {
		if _, has := c.data[key]; has {
			return true
		}
		if _, has := c.newData[key]; has {
			return true
		}
		c.newData[key] = true
	}
	return false
}

func (c *Cache) Dump() error {
	if c.useRedis {
		return nil
	}

	cacheFile, err := os.OpenFile(c.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0660)
	if err != nil {
		return err
	}
	defer cacheFile.Close()

	if err = syscall.Flock(int(cacheFile.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}

	writer := bufio.NewWriter(cacheFile)
	for key, _ := range c.newData {
		if _, err = writer.WriteString(key); err != nil {
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
