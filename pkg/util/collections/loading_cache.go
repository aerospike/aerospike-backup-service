package collections

import (
	"context"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
)

// LoadFunc is the LoadingCache value loader.
type LoadFunc[K comparable, T any] func(context.Context, K) (T, error)

// Cache is a generic cache interface.
type Cache[K comparable, T any] interface {
	GetWithContext(ctx context.Context, key K) (T, error)
}

// LoadingCache maps keys to values where values are automatically loaded by the cache.
type LoadingCache[K comparable, T any] struct {
	data     sync.Map
	loadFunc LoadFunc[K, T]
	ttl      *time.Duration // nil means cache forever, 0 means no cache.
}
type cacheItem[T any] struct {
	value     T
	expiresAt *time.Time // nil when ttl is nil.
	loaded    bool       // indicates if the item was successfully loaded.
	mu        sync.Mutex
}

func NewLoadingCache[K comparable, T any](
	ctx context.Context,
	loadFunc LoadFunc[K, T],
	ttl *time.Duration,
) *LoadingCache[K, T] {
	c := &LoadingCache[K, T]{
		loadFunc: loadFunc,
		ttl:      ttl,
	}

	if ttl != nil && *ttl > 0 {
		go c.startCleanup(ctx)
	}

	return c
}

func (c *LoadingCache[K, T]) GetWithContext(ctx context.Context, key K) (T, error) {
	val, ok := c.data.Load(key)
	if !ok { // new key
		val, _ = c.data.LoadOrStore(key, &cacheItem[T]{})
	}
	item := val.(*cacheItem[T])

	item.mu.Lock()
	defer item.mu.Unlock()

	now := time.Now()

	// Check if the cached value is still valid.
	if item.loaded {
		if c.ttl == nil {
			return item.value, nil
		}
		if item.expiresAt != nil && now.Before(*item.expiresAt) {
			item.expiresAt = ptr.Of(now.Add(*c.ttl))
			return item.value, nil
		}
	}

	// The item is expired or missing from the cache. Load a new value.
	loadedValue, err := c.loadFunc(ctx, key)
	if err != nil {
		return item.value, err
	}

	item.value = loadedValue
	item.loaded = true

	if c.ttl != nil {
		item.expiresAt = ptr.Of(now.Add(*c.ttl))
	}

	return loadedValue, nil
}

func (c *LoadingCache[K, T]) startCleanup(ctx context.Context) {
	interval := *c.ttl
	if interval > time.Hour { // We don't want to trigger cleanup too often.
		interval = time.Hour
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.deleteExpired()
		case <-ctx.Done():
			return
		}
	}
}

func (c *LoadingCache[K, T]) deleteExpired() {
	now := time.Now()
	c.data.Range(func(key, value any) bool {
		item := value.(*cacheItem[T])

		item.mu.Lock()
		expired := item.loaded && item.expiresAt != nil && now.After(*item.expiresAt)
		item.mu.Unlock()

		if expired {
			c.data.Delete(key)
		}
		return true
	})
}
