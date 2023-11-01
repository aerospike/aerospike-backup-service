package util

import (
	"sync"
	"time"
)

type LoadFunc func(string) (any, error)

type Cache struct {
	sync.Mutex
	data     map[string]any
	cleanCh  chan bool
	stopCh   chan bool
	loadFunc LoadFunc
}

func NewCache(loadFunc LoadFunc) *Cache {
	cache := &Cache{
		data:     make(map[string]any),
		cleanCh:  make(chan bool),
		stopCh:   make(chan bool),
		loadFunc: loadFunc,
	}

	go cache.startCleanup()

	return cache
}

func (c *Cache) Get(key string) (any, error) {
	c.Lock()
	defer c.Unlock()

	val, found := c.data[key]

	if found {
		return val, nil
	}

	loadedValue, err := c.loadFunc(key)
	if err != nil {
		return nil, err
	}

	c.data[key] = loadedValue
	return loadedValue, nil
}

func (c *Cache) startCleanup() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.clean()
		case <-c.stopCh:
			ticker.Stop()
			return
		}
	}
}

func (c *Cache) clean() {
	c.Lock()
	defer c.Unlock()
	c.data = make(map[string]interface{})
}

func (c *Cache) StopCleanup() {
	close(c.stopCh)
}
