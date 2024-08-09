package util

import (
	"context"
	"sync"
	"time"
)

// LoadFunc is the LoadingCache value loader.
type LoadFunc[K comparable, T any] func(K) (T, error)

// LoadingCache maps keys to values where values are automatically
// loaded by the cache.
type LoadingCache[K comparable, T any] struct {
	sync.Mutex
	ctx      context.Context
	data     map[K]T
	loadFunc LoadFunc[K, T]
}

// NewLoadingCache returns a new LoadingCache instance.
func NewLoadingCache[K comparable, T any](ctx context.Context,
	loadFunc LoadFunc[K, T]) *LoadingCache[K, T] {
	cache := &LoadingCache[K, T]{
		ctx:      ctx,
		data:     make(map[K]T),
		loadFunc: loadFunc,
	}

	go cache.startCleanup()
	return cache
}

// Get retrieves or loads the value for the specified key and stores
// it in the cache.
func (c *LoadingCache[K, T]) Get(key K) (T, error) {
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

func (c *LoadingCache[T, K]) startCleanup() {
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

func (c *LoadingCache[K, T]) clean() {
	c.Lock()
	defer c.Unlock()
	c.data = make(map[K]T)
}
