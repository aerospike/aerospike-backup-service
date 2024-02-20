package service

import (
	"fmt"
	"testing"
	"time"
)

func Test_timer(t *testing.T) {
	r := NewRetryService("test")
	r.retry(func() error {
		return fmt.Errorf("")
	}, time.Second, 3)
	time.Sleep(50 * time.Second)
}
