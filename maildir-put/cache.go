package main

import (
	"bufio"
	"io"
	"os"
	"syscall"
)

type Cache struct {
	data    map[string]bool
	newData map[string]bool
	path    string
}

func OpenCache(path string) (c *Cache, err error) {
	var key string

	c = &Cache{make(map[string]bool), make(map[string]bool), path}

	cacheFile, err := os.Open(c.path)
	if err != nil {
		if os.IsNotExist(err) {
			err = nil
		} else {
			c = nil
		}
		return
	}

	reader := bufio.NewReader(cacheFile)
	for err != io.EOF {
		if key, err = reader.ReadString('\n'); err != nil && err != io.EOF {
			c = nil
			return
		}
		if key != "" {
			c.data[key[:len(key)-1]] = true
		}
	}

	err = nil
	return
}

func (c *Cache) Set(key string) {
	c.newData[key] = true
}

func (c *Cache) Get(key string) bool {
	_, has := c.data[key]
	if !has {
		_, has = c.newData[key]
	}
	return has
}

func (c *Cache) Dump() error {
	cacheFile, err := os.OpenFile(c.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0660)
	if err != nil {
		return err
	}
	defer cacheFile.Close()

	if err = syscall.Flock(int(cacheFile.Fd()), 2); err != nil {
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
