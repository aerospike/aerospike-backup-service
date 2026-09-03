package collections

import (
	"context"
	"sync"
	"time"
)

// Cache is the interface for a context-aware cache.
type Cache[K comparable, T any] interface {
	// Get returns the value for the given key.
	Get(ctx context.Context, key K) (T, error)
}

type LoadFunc[K comparable, T any] func(context.Context, K) (T, error)

type cacheItem[T any] struct {
	sync.Mutex

	value       T
	loaded      bool // true once a value has been cached for this key
	expireTimer *time.Timer
}

var _ Cache[string, any] = (*LoadingCache[string, any])(nil)

// LoadingCache handles storage, locking, loading, and expiration logic.
type LoadingCache[K comparable, T any] struct {
	data     sync.Map
	ttl      *time.Duration
	loadFunc LoadFunc[K, T]
}

func NewLoadingCache[K comparable, T any](
	loadFunc LoadFunc[K, T],
	ttl *time.Duration,
) *LoadingCache[K, T] {
	return &LoadingCache[K, T]{
		ttl:      ttl,
		loadFunc: loadFunc,
	}
}

// Get returns the cached value for key, or creates one via loadFunc on cache miss.
func (c *LoadingCache[K, T]) Get(ctx context.Context, key K) (T, error) {
	// Bypass cache if TTL is explicitly 0
	if c.ttl != nil && *c.ttl == 0 {
		return c.loadFunc(ctx, key)
	}

	val, ok := c.data.Load(key)
	if !ok {
		val, _ = c.data.LoadOrStore(key, &cacheItem[T]{})
	}
	item := val.(*cacheItem[T])

	item.Lock()
	defer item.Unlock()

	// Cache hit.
	if item.loaded {
		return item.value, nil
	}

	// Cache miss: create a new value.
	value, err := c.loadFunc(ctx, key)
	if err != nil {
		var zeroValue T
		return zeroValue, err
	}

	item.value = value
	item.loaded = true
	if c.ttl != nil {
		c.scheduleExpiry(key, item)
	}

	return value, nil
}

func (c *LoadingCache[K, T]) scheduleExpiry(key K, item *cacheItem[T]) {
	if item.expireTimer != nil {
		item.expireTimer.Stop()
	}

	item.expireTimer = time.AfterFunc(*c.ttl, func() {
		c.data.Delete(key)
	})
}
