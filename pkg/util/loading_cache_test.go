package util

import (
	"context"
	"strconv"
	"testing"
)

func TestLoadingCache(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cache := NewLoadingCache(ctx, func(s string) any {
		atoi, _ := strconv.Atoi(s)
		return atoi
	})
	value, _ := cache.Get("1")
	if value != 1 {
		t.Error("The value is expected to be 1")
	}
	value, _ = cache.Get("1") // existing
	if value != 1 {
		t.Error("The value is expected to be 1")
	}
	go cache.startCleanup()
	cancel()
	cache.clean()
	value, _ = cache.Get("2")
	if value != 2 {
		t.Error("The value is expected to be 2")
	}
}
