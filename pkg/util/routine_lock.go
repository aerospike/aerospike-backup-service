package util

import "sync"

type LockMap struct {
	locks sync.Map // map[string]*sync.Mutex
}

func (rl *LockMap) Get(key string) *sync.RWMutex {
	actual, _ := rl.locks.LoadOrStore(key, &sync.RWMutex{})
	return actual.(*sync.RWMutex)
}
