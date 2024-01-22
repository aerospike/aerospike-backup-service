package util

import (
	"context"
	"sync"
	"time"
)

// LoadFunc is the LoadingCache value loader.
type LoadFunc[T any] func(string) (T, error)

// LoadingCache maps keys to values where values are automatically
// loaded by the cache.
type LoadingCache[T any] struct {
	sync.Mutex
	ctx      context.Context
	data     map[string]T
	loadFunc LoadFunc[T]
}

// NewLoadingCache returns a new LoadingCache instance.
func NewLoadingCache[T any](ctx context.Context, loadFunc LoadFunc[T]) *LoadingCache[T] {
	cache := &LoadingCache[T]{
		ctx:      ctx,
		data:     make(map[string]T),
		loadFunc: loadFunc,
	}

	go cache.startCleanup()
	return cache
}

// Get retrieves or loads the value for the specified key and stores
// it in the cache.
func (c *LoadingCache[T]) Get(key string) (T, error) {
	c.Lock()
	defer c.Unlock()

	val, found := c.data[key]
	if found {
		return val, nil
	}

	loadedValue, err := c.loadFunc(key)
	if err != nil {
		return loadedValue, err
	}

	c.data[key] = loadedValue
	return loadedValue, nil
}

func (c *LoadingCache[T]) startCleanup() {
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

func (c *LoadingCache[T]) clean() {
	c.Lock()
	defer c.Unlock()
	c.data = make(map[string]T)
}
