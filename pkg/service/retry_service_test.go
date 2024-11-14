package service

import (
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/require"
)

var r = NewRetryService(models.RetryPolicy{
	MaxRetries:  2,
	BaseTimeout: 100 * time.Millisecond,
	Multiplier:  1,
}, slog.Default())

func Test_timer(t *testing.T) {
	counterLock := sync.Mutex{}
	retryCounter := 2
	err := r.retry("test", func() error {
		counterLock.Lock()
		defer counterLock.Unlock()
		if retryCounter > 0 {
			retryCounter--
			return errors.New("mock error")
		}
		return nil
	})
	require.NoError(t, err)

	time.Sleep(1 * time.Second)
	counterLock.Lock()
	defer counterLock.Unlock()
	if retryCounter != 0 {
		t.Errorf("Expected retryCounter 0, got %d", retryCounter)
	}
}

func Test_timer_expires(t *testing.T) {
	counterLock := sync.Mutex{}
	retryCounter := 0
	const attempts = 3
	_ = r.retry("test", func() error {
		counterLock.Lock()
		defer counterLock.Unlock()
		retryCounter++
		return errors.New("mock error")
	})

	time.Sleep(1 * time.Second)
	counterLock.Lock()
	defer counterLock.Unlock()
	if retryCounter != attempts {
		t.Errorf("Expected retryCounter %d, got %d", attempts, retryCounter)
	}
}

func Test_timerRunTwice(t *testing.T) {
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
	_ = r.retry("test", f)
	_ = r.retry("test", f)

	time.Sleep(1 * time.Second)
	counterLock.Lock()
	defer counterLock.Unlock()
	if retryCounter != 0 {
		t.Errorf("Expected retryCounter 0, got %d", retryCounter)
	}
}
