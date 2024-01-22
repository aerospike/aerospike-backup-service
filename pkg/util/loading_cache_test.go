package util

import (
	"context"
	"errors"
	"strconv"
	"testing"
)

func TestLoadingCache(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cache := NewLoadingCache(ctx, func(s string) (int, error) {
		return strconv.Atoi(s)
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

func TestLoadingCache_Error(t *testing.T) {
	cache := NewLoadingCache(context.Background(), func(s string) (any, error) {
		return nil, errors.New("error")
	})
	_, err := cache.Get("1")
	if err == nil {
		t.Error("Error must not be nil")
	}
}
