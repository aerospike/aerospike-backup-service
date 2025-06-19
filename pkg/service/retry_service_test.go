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

var r = newRetryExecutor(models.RetryPolicy{
	MaxRetries:  2,
	BaseTimeout: 100 * time.Millisecond,
	Multiplier:  1,
}, slog.Default())

func Test_timer(t *testing.T) {
	counterLock := sync.Mutex{}
	retryCounter := 2
	err := r.run("test", func() error {
		counterLock.Lock()
		defer counterLock.Unlock()
		if retryCounter > 0 {
			retryCounter--
			return errors.New("mock error")
		}
		return nil
	}, func() {})
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
	_ = r.run("test", func() error {
		counterLock.Lock()
		defer counterLock.Unlock()
		retryCounter++
		return errors.New("mock error")
	}, func() {})

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
	_ = r.run("test", f, func() {})
	_ = r.run("test", f, func() {})

	time.Sleep(1 * time.Second)
	counterLock.Lock()
	defer counterLock.Unlock()
	if retryCounter != 0 {
		t.Errorf("Expected retryCounter 0, got %d", retryCounter)
	}
}

func Test_retry_attempts_expected_count(t *testing.T) {
	attempts := 0
	expectedAttempts := 3 // MaxRetries=2 + 1 initial attempt

	err := r.run("retry-test", func() error {
		attempts++
		return errors.New("still failing")
	}, func() {})

	require.Error(t, err)
	require.Equal(t, expectedAttempts, attempts, "Function was not retried the expected number of times")
}
