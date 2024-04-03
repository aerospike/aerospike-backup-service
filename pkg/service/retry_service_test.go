package service

import (
	"errors"
	"sync"
	"testing"
	"time"
)

const timeout = 100 * time.Millisecond

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
	}, timeout, 3)

	time.Sleep(1 * time.Second)
	counterLock.Lock()
	defer counterLock.Unlock()
	if retryCounter != 0 {
		t.Errorf("Expected retryCounter 0, got %d", retryCounter)
	}
}

func Test_timer_expires(t *testing.T) {
	r := NewRetryService("test")
	counterLock := sync.Mutex{}
	retryCounter := 0
	const attempts = 3
	r.retry(func() error {
		counterLock.Lock()
		defer counterLock.Unlock()
		retryCounter++
		return errors.New("mock error")
	}, timeout, attempts-1)

	time.Sleep(1 * time.Second)
	counterLock.Lock()
	defer counterLock.Unlock()
	if retryCounter != attempts {
		t.Errorf("Expected retryCounter %d, got %d", attempts, retryCounter)
	}
}

func Test_timerRunTwice(t *testing.T) {
	r := NewRetryService("test")
	counterLock := sync.Mutex{}
	retryCounter := 3
	f := func() error {
		counterLock.Lock()
		defer counterLock.Unlock()
		if retryCounter > 0 {
			retryCounter--
			return errors.New("mock error")
		}
		return nil
	}
	r.retry(f, timeout, 3)
	r.retry(f, timeout, 3)

	time.Sleep(1 * time.Second)
	counterLock.Lock()
	defer counterLock.Unlock()
	if retryCounter != 0 {
		t.Errorf("Expected retryCounter 0, got %d", retryCounter)
	}
}
