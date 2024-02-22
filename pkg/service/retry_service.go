package service

import (
	"log/slog"
	"sync"
	"time"
)

// RetryService a service for retrying a function with a specified interval and number of attempts.
type RetryService struct {
	label string
	timer *time.Timer
	mu    sync.Mutex
}

// NewRetryService returns a new RetryService instance.
// label is used for logging purposes only.
func NewRetryService(label string) *RetryService {
	return &RetryService{
		label: label,
	}
}

func (r *RetryService) retry(f func() error, retryInterval time.Duration, n int32) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.clearTimer()
	err := f()

	if err == nil { // function executed successfully, no retry needed
		return
	}

	if n == 0 {
		slog.Warn("Execution failed, no retry attempts left", "label", r.label, "err", err)
		return
	}

	slog.Info("Execution failed, retry scheduled", "label", r.label, "retryInterval", retryInterval, "err", err)
	r.timer = time.AfterFunc(retryInterval, func() {
		r.retry(f, retryInterval, n-1)
	})
}

func (r *RetryService) clearTimer() {
	if r.timer != nil {
		r.timer.Stop()
		if r.timer.C != nil {
			<-r.timer.C
		}
	}
}
