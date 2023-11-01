package util

import (
	"context"
	"sync"
	"time"
)

type LoadFunc func(string) (any, error)

type LoadingCache struct {
	sync.Mutex
	data     map[string]any
	cleanCh  chan bool
	loadFunc LoadFunc
	ctx      context.Context
}

// NewLoadingCache returns new LoadingCache instance
func NewLoadingCache(ctx context.Context, loadFunc LoadFunc) *LoadingCache {
	cache := &LoadingCache{
		data:     make(map[string]any),
		cleanCh:  make(chan bool),
		loadFunc: loadFunc,
		ctx:      ctx,
	}

	go cache.startCleanup()
	return cache
}

// Get retrieves or loads the value for the specified key and saves it in the cache.
func (c *LoadingCache) Get(key string) (any, error) {
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

func (c *LoadingCache) startCleanup() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.clean()
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *LoadingCache) clean() {
	c.Lock()
	defer c.Unlock()
	c.data = make(map[string]any)
}
