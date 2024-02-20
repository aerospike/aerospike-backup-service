package service

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func Test_timer(t *testing.T) {
	r := NewRetryService("test")
	counterLock := sync.Mutex{}
	retryCounter := 2
	r.retry(func() error {
		counterLock.Lock()
		defer counterLock.Unlock()
		if retryCounter > 0 {
			retryCounter--
			return errors.New("mock error")
		}
		return nil
	}, time.Second, 3)

	time.Sleep(5 * time.Second)
	counterLock.Lock()
	defer counterLock.Unlock()
	if retryCounter != 0 {
		t.Errorf("Expected retryCounter 0, got %d", retryCounter)
	}
}
