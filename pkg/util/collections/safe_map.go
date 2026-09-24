package collections

import "sync"

// SafeMap is a thread-safe map with generic key and value types.
// It is backed by sync.Map, so no method holds a lock while running a caller's callback: an
// Iterate or Find callback may lock the values it visits and may call any method of the map.
type SafeMap[K comparable, V any] struct {
	m sync.Map
}

// NewSafeMap creates a new SafeMap instance.
func NewSafeMap[K comparable, V any]() *SafeMap[K, V] {
	return &SafeMap[K, V]{}
}

// Load retrieves a value by key.
func (s *SafeMap[K, V]) Load(key K) (V, bool) {
	var zeroValue V
	if s == nil {
		return zeroValue, false
	}

	value, ok := s.m.Load(key)
	if !ok {
		return zeroValue, false
	}

	return cast[V](value), true
}

// LoadOrStore retrieves a value by key if it exists; otherwise, it stores the provided default value.
func (s *SafeMap[K, V]) LoadOrStore(key K, defaultValue V) V {
	if s == nil {
		var zeroValue V
		return zeroValue
	}

	actual, _ := s.m.LoadOrStore(key, defaultValue)

	return cast[V](actual)
}

// Store inserts or updates a key-value pair.
func (s *SafeMap[K, V]) Store(key K, value V) {
	s.m.Store(key, value)
}

// Remove deletes the value for a key and returns whether the key was present.
func (s *SafeMap[K, V]) Remove(key K) bool {
	_, loaded := s.m.LoadAndDelete(key)
	return loaded
}

// Iterate calls callback for every key-value pair in the map.
// The iteration is not a snapshot: pairs stored or removed while it runs, including by callback
// itself, may or may not be visited.
func (s *SafeMap[K, V]) Iterate(callback func(key K, value V)) {
	s.m.Range(func(key, value any) bool {
		callback(cast[K](key), cast[V](value))
		return true
	})
}

// Find returns the first key-value pair whose value satisfies match.
func (s *SafeMap[K, V]) Find(match func(value V) bool) (K, V, bool) {
	var (
		foundKey   K
		foundValue V
		found      bool
	)

	s.m.Range(func(key, value any) bool {
		v := cast[V](value)
		if !match(v) {
			return true
		}

		foundKey, foundValue, found = cast[K](key), v, true

		return false
	})

	return foundKey, foundValue, found
}

// Size returns the number of key-value pairs in the map. Pairs stored or removed while it counts
// may or may not be included.
func (s *SafeMap[K, V]) Size() int {
	size := 0
	s.m.Range(func(any, any) bool {
		size++
		return true
	})

	return size
}

// cast converts a value read from the sync.Map back to its static type. Only values of type T are
// ever stored, but a nil interface value comes back as an untyped nil, which the two-value
// assertion turns into T's zero value instead of panicking.
func cast[T any](value any) T {
	typed, _ := value.(T)
	return typed
}
