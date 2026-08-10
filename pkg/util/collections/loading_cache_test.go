package collections

import (
	"context"
	"errors"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadingCache_GetWithContext(t *testing.T) {
	ctx := t.Context()
	var callCount atomic.Int32
	loadFunc := func(_ context.Context, s string) (int, error) {
		callCount.Add(1)
		return strconv.Atoi(s)
	}

	ttl := time.Minute
	cache := NewLoadingCacheContext(ctx, loadFunc, &ttl)

	// First call
	val, err := cache.Get(ctx, "1")
	require.NoError(t, err)
	assert.Equal(t, 1, val)
	assert.Equal(t, int32(1), callCount.Load())

	// Second call - should be cached
	val, err = cache.Get(ctx, "1")
	require.NoError(t, err)
	assert.Equal(t, 1, val)
	assert.Equal(t, int32(1), callCount.Load())

	// Different key
	val, err = cache.Get(ctx, "2")
	require.NoError(t, err)
	assert.Equal(t, 2, val)
	assert.Equal(t, int32(2), callCount.Load())
}

func TestLoadingCache_Forever(t *testing.T) {
	ctx := t.Context()
	var callCount atomic.Int32
	loadFunc := func(_ context.Context, s string) (int, error) {
		callCount.Add(1)
		return strconv.Atoi(s)
	}

	cache := NewLoadingCacheContext(ctx, loadFunc, nil)

	// First call
	val, err := cache.Get(ctx, "1")
	require.NoError(t, err)
	assert.Equal(t, 1, val)
	assert.Equal(t, int32(1), callCount.Load())

	// Second call - should be cached forever
	val, err = cache.Get(ctx, "1")
	require.NoError(t, err)
	assert.Equal(t, 1, val)
	assert.Equal(t, int32(1), callCount.Load())
}

func TestLoadingCache_Expiry(t *testing.T) {
	ctx := t.Context()
	var callCount atomic.Int32
	loadFunc := func(_ context.Context, s string) (int, error) {
		callCount.Add(1)
		return strconv.Atoi(s)
	}

	ttl := 50 * time.Millisecond
	cache := NewLoadingCacheContext(ctx, loadFunc, &ttl)

	// First call
	val, err := cache.Get(ctx, "1")
	require.NoError(t, err)
	assert.Equal(t, 1, val)
	assert.Equal(t, int32(1), callCount.Load())

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Second call - should reload
	val, err = cache.Get(ctx, "1")
	require.NoError(t, err)
	assert.Equal(t, 1, val)
	assert.Equal(t, int32(2), callCount.Load())
}

func TestLoadingCache_Error(t *testing.T) {
	ctx := t.Context()
	loadFunc := func(_ context.Context, _ string) (int, error) {
		return 0, errors.New("load error")
	}

	ttl := time.Minute
	cache := NewLoadingCacheContext(ctx, loadFunc, &ttl)

	val, err := cache.Get(ctx, "1")
	require.Error(t, err)
	assert.Equal(t, "load error", err.Error())
	assert.Equal(t, 0, val)

	// Try again, should try loading again (and fail again)
	_, err = cache.Get(ctx, "1")
	assert.Error(t, err)
}

func TestLoadingCache_ZeroTTL(t *testing.T) {
	ctx := t.Context()
	var callCount atomic.Int32
	loadFunc := func(_ context.Context, s string) (int, error) {
		callCount.Add(1)
		return strconv.Atoi(s)
	}

	ttl := time.Duration(0)
	cache := NewLoadingCacheContext(ctx, loadFunc, &ttl)

	// First call
	val, err := cache.Get(ctx, "1")
	require.NoError(t, err)
	assert.Equal(t, 1, val)
	assert.Equal(t, int32(1), callCount.Load())

	// Second call - should reload because TTL is 0
	val, err = cache.Get(ctx, "1")
	require.NoError(t, err)
	assert.Equal(t, 1, val)
	assert.Equal(t, int32(2), callCount.Load())
}

func TestLoadingCache_AccessExtendsTTL(t *testing.T) {
	ctx := t.Context()
	var callCount atomic.Int32
	loadFunc := func(_ context.Context, s string) (int, error) {
		callCount.Add(1)
		return strconv.Atoi(s)
	}

	ttl := 500 * time.Millisecond
	cache := NewLoadingCacheContext(ctx, loadFunc, &ttl)

	// First call
	_, err := cache.Get(ctx, "1")
	require.NoError(t, err)
	assert.Equal(t, int32(1), callCount.Load())

	// Access repeatedly to keep it alive.
	// Total time is ~500ms (> TTL), refreshed every 100ms
	for range 5 {
		time.Sleep(100 * time.Millisecond)
		_, err := cache.Get(ctx, "1")
		require.NoError(t, err)
	}

	// Should not have reloaded
	assert.Equal(t, int32(1), callCount.Load())

	// Now wait longer than TTL
	time.Sleep(600 * time.Millisecond)

	// Should reload
	_, err = cache.Get(ctx, "1")
	require.NoError(t, err)
	assert.Equal(t, int32(2), callCount.Load())
}

func TestBaseCache_deleteExpired(t *testing.T) {
	ctx := t.Context()
	ttl := time.Minute
	cache := newBaseCache[string, int](ctx, &ttl)

	past := time.Now().Add(-time.Hour)
	item := &cacheItem[int]{
		value:     42,
		loaded:    true,
		expiresAt: &past,
	}
	cache.data.Store("expired", item)

	future := time.Now().Add(time.Hour)
	active := &cacheItem[int]{
		value:     7,
		loaded:    true,
		expiresAt: &future,
	}
	cache.data.Store("active", active)

	cache.deleteExpired()

	_, expiredOK := cache.data.Load("expired")
	assert.False(t, expiredOK)

	_, activeOK := cache.data.Load("active")
	assert.True(t, activeOK)
}
