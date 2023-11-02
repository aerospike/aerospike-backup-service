package util

import (
	"context"
	"sync"
	"time"
)

// LoadFunc is the LoadingCache value loader.
type LoadFunc func(string) (any, error)

// LoadingCache maps keys to values where values are automatically
// loaded by the cache.
type LoadingCache struct {
	sync.Mutex
	ctx      context.Context
	data     map[string]any
	loadFunc LoadFunc
}

// NewLoadingCache returns a new LoadingCache instance.
func NewLoadingCache(ctx context.Context, loadFunc LoadFunc) *LoadingCache {
	cache := &LoadingCache{
		ctx:      ctx,
		data:     make(map[string]any),
		loadFunc: loadFunc,
	}

	go cache.startCleanup()
	return cache
}

// Get retrieves or loads the value for the specified key and stores
// it in the cache.
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
