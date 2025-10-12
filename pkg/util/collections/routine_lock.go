package collections

import "sync"

type LockMap struct {
	locks sync.Map // map[string]*sync.Mutex
}

// Get returns the existing mutex for the key if present.
// Otherwise, it creates, stores and returns new mutex.
func (rl *LockMap) Get(key string) *sync.RWMutex {
	actual, _ := rl.locks.LoadOrStore(key, &sync.RWMutex{})
	return actual.(*sync.RWMutex)
}
