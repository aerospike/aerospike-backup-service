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

func TestSafeMap_Apply(t *testing.T) {
	m := NewSafeMap[string, int]()
	m.Store("count", 1)

	called := false
	m.Apply("count", func(v int) {
		called = true
		assert.Equal(t, 1, v)
	})
	assert.True(t, called)

	m.Apply("missing", func(int) {
		require.FailNow(t, "callback should not run for missing key")
	})
}

func TestSafeMap_ApplyOrCreate(t *testing.T) {
	m := NewSafeMap[string, int]()
	m.Store("existing", 5)

	seen := 0
	m.ApplyOrCreate("existing", func(v int) {
		seen = v
	}, 99)
	assert.Equal(t, 5, seen)

	m.ApplyOrCreate("new", func(int) {
		require.FailNow(t, "callback should not run when key is created")
	}, 7)
	val, ok := m.Load("new")
	require.True(t, ok)
	assert.Equal(t, 7, val)
}

func TestSafeMap_IterateReplaceContentSize(t *testing.T) {
	m := NewSafeMap[string, int]()
	m.Store("a", 1)
	m.Store("b", 2)

	seen := make(map[string]int)
	m.Iterate(func(key string, value int) {
		seen[key] = value
	})
	assert.Equal(t, map[string]int{"a": 1, "b": 2}, seen)
	assert.Equal(t, 2, m.Size())

	m.ReplaceContent(map[string]int{"c": 3})
	assert.Equal(t, 1, m.Size())
	val, ok := m.Load("c")
	require.True(t, ok)
	assert.Equal(t, 3, val)
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
