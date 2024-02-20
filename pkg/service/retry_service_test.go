package service

import (
	"errors"
	"testing"
	"time"
)

func Test_timer(t *testing.T) {
	r := NewRetryService("test")
	retryCounter := 2
	r.retry(func() error {
		if retryCounter > 0 {
			retryCounter--
			return errors.New("mock error")
		}
		return nil
	}, time.Second, 3)
	time.Sleep(5 * time.Second)
	if retryCounter != 0 {
		t.Errorf("Expected retryCounter 0, got %d", retryCounter)
	}
}
