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
	if s == nil {
		var zeroValue V
		return zeroValue, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.m[key]
	return value, ok
}

// Store inserts or updates a key-value pair.
func (s *SafeMap[K, V]) Store(key K, value V) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
}

// Apply applies a function to a specific key if it exists.
func (s *SafeMap[K, V]) Apply(key K, callback func(value V)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if v, exists := s.m[key]; exists {
		callback(v)
	}
}

// Iterate iterates over all key-value pairs in the map.
func (s *SafeMap[K, V]) Iterate(callback func(key K, value V)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for key, value := range s.m {
		callback(key, value)
	}
}

// ReplaceContent replaces the content of the map with a new one.
func (s *SafeMap[K, V]) ReplaceContent(newMap map[K]V) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m = newMap
}

// Size returns the number of elements in the map.
func (s *SafeMap[K, V]) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.m)
}
