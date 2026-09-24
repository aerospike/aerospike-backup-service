package collections

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSafeMap_NilReceiver(t *testing.T) {
	var m *SafeMap[string, int]

	val, ok := m.Load("key")
	assert.False(t, ok)
	assert.Equal(t, 0, val)
	assert.Equal(t, 0, m.LoadOrStore("key", 42))
}

func TestSafeMap_StoreLoadRemove(t *testing.T) {
	m := NewSafeMap[string, int]()

	m.Store("a", 1)
	val, ok := m.Load("a")
	require.True(t, ok)
	assert.Equal(t, 1, val)

	assert.False(t, m.Remove("missing"))
	assert.True(t, m.Remove("a"))
	_, ok = m.Load("a")
	assert.False(t, ok)
}

func TestSafeMap_LoadOrStore(t *testing.T) {
	m := NewSafeMap[string, int]()

	assert.Equal(t, 10, m.LoadOrStore("k", 10))
	assert.Equal(t, 10, m.LoadOrStore("k", 99))
}

func TestSafeMap_NilInterfaceValue(t *testing.T) {
	m := NewSafeMap[string, error]()
	m.Store("none", nil)

	val, ok := m.Load("none")
	require.True(t, ok)
	assert.NoError(t, val)
	assert.NoError(t, m.LoadOrStore("none", assert.AnError))
}

func TestSafeMap_IterateSize(t *testing.T) {
	m := NewSafeMap[string, int]()
	m.Store("a", 1)
	m.Store("b", 2)

	seen := make(map[string]int)
	m.Iterate(func(key string, value int) {
		seen[key] = value
	})
	assert.Equal(t, map[string]int{"a": 1, "b": 2}, seen)
	assert.Equal(t, 2, m.Size())
}

// The callback may modify the map it is iterating over.
func TestSafeMap_IterateCallbackRemoves(t *testing.T) {
	m := NewSafeMap[string, int]()
	m.Store("a", 1)
	m.Store("b", 2)

	m.Iterate(func(key string, _ int) {
		m.Remove(key)
	})
	assert.Zero(t, m.Size())
}

func TestSafeMap_Find(t *testing.T) {
	m := NewSafeMap[string, int]()
	m.Store("a", 1)
	m.Store("b", 2)

	key, val, found := m.Find(func(value int) bool { return value == 2 })
	require.True(t, found)
	assert.Equal(t, "b", key)
	assert.Equal(t, 2, val)

	key, val, found = m.Find(func(int) bool { return false })
	assert.False(t, found)
	assert.Empty(t, key)
	assert.Zero(t, val)
}

func TestSafeMap_ConcurrentLoadOrStore(t *testing.T) {
	m := NewSafeMap[int, int]()
	const goroutines = 32

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := range goroutines {
		go func(key int) {
			defer wg.Done()
			m.LoadOrStore(key%4, key)
			m.Store(key%4, key)
		}(i)
	}
	wg.Wait()

	assert.Equal(t, 4, m.Size())
}
