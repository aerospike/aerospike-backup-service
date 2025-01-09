package util

import "sync"

// SafeMap is a thread-safe map with generic key and value types.
type SafeMap[K comparable, V any] struct {
	mu sync.RWMutex
	m  map[K]V
}

// NewSafeMap creates a new SafeMap instance.
func NewSafeMap[K comparable, V any]() *SafeMap[K, V] {
	return &SafeMap[K, V]{
		m: make(map[K]V),
	}
}

// Load retrieves a value by key.
func (s *SafeMap[K, V]) Load(key K) (V, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.m[key]
	return value, ok
}

// Iterate iterates over all key-value pairs in the map.
func (s *SafeMap[K, V]) Iterate(callback func(key K, value V)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for key, value := range s.m {
		callback(key, value)
	}
}

func (s *SafeMap[K, V]) ReplaceContent(newMap map[K]V) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m = newMap
}
