package collections

import (
	"context"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
)

// Cache is the interface for a simple, context-free cache.
type Cache[K comparable, T any] interface {
	// Get returns the value for the given key.
	Get(key K) (T, error)
}

// CacheContext is the interface for a context-aware cache.
type CacheContext[K comparable, T any] interface {
	// Get returns the value for the given key.
	Get(ctx context.Context, key K) (T, error)
}

type LoadFunc[K comparable, T any] func(K) (T, error)
type LoadFuncContext[K comparable, T any] func(context.Context, K) (T, error)

// provider is an internal helper to wrap the specific loading logic.
type provider[T any] func() (T, error)

type cacheItem[T any] struct {
	sync.Mutex

	value     T
	expiresAt *time.Time
	loaded    bool // true if the value has been successfully loaded at least once
}

// baseCache handles storage, locking, and expiration logic.
// It is agnostic to the context or loading signature.
type baseCache[K comparable, T any] struct {
	data sync.Map
	ttl  *time.Duration
}

func newBaseCache[K comparable, T any](ctx context.Context, ttl *time.Duration) *baseCache[K, T] {
	b := &baseCache[K, T]{
		ttl: ttl,
	}

	if ttl != nil && *ttl > 0 {
		go b.startCleanup(ctx)
	}

	return b
}

// fetch manages the retrieval, locking, and reloading of cache items.
func (c *baseCache[K, T]) fetch(key K, loader provider[T]) (T, error) {
	val, ok := c.data.Load(key)
	if !ok {
		val, _ = c.data.LoadOrStore(key, &cacheItem[T]{})
	}
	item := val.(*cacheItem[T])

	item.Lock()
	defer item.Unlock()

	now := time.Now()

	// 1. Check if the cached value is already valid
	if item.loaded {
		if c.ttl == nil {
			return item.value, nil
		}
		if item.expiresAt != nil && now.Before(*item.expiresAt) {
			item.expiresAt = ptr.Of(now.Add(*c.ttl))
			return item.value, nil
		}
	}

	// 2. Value is missing or expired; execute the loader
	loadedValue, err := loader()
	if err != nil {
		var zeroValue T
		return zeroValue, err
	}

	// 3. Update the item
	item.value = loadedValue
	item.loaded = true
	if c.ttl != nil {
		item.expiresAt = ptr.Of(now.Add(*c.ttl))
	}

	return loadedValue, nil
}

func (c *baseCache[K, T]) startCleanup(ctx context.Context) {
	interval := max(*c.ttl, time.Hour) // We don't want to trigger cleanup too often.

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

func (c *baseCache[K, T]) deleteExpired() {
	now := time.Now()
	c.data.Range(func(key, value any) bool {
		item := value.(*cacheItem[T])

		item.Lock()
		expired := item.loaded && item.expiresAt != nil && now.After(*item.expiresAt)
		item.Unlock()

		if expired {
			c.data.Delete(key)
		}
		return true
	})
}

// --- Implementation 1: Context-Aware Cache ---

var _ CacheContext[string, any] = (*LoadingCacheContext[string, any])(nil)

type LoadingCacheContext[K comparable, T any] struct {
	base     *baseCache[K, T]
	loadFunc LoadFuncContext[K, T]
}

func NewLoadingCacheContext[K comparable, T any](
	ctx context.Context, // Required for background cleanup goroutine
	loadFunc LoadFuncContext[K, T],
	ttl *time.Duration,
) *LoadingCacheContext[K, T] {
	return &LoadingCacheContext[K, T]{
		base:     newBaseCache[K, T](ctx, ttl),
		loadFunc: loadFunc,
	}
}

// Get returns the value for the given key, passing the context to the loader if a reload is needed.
func (c *LoadingCacheContext[K, T]) Get(ctx context.Context, key K) (T, error) {
	return c.base.fetch(key, func() (T, error) {
		return c.loadFunc(ctx, key)
	})
}
