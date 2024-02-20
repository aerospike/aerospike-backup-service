package service

import (
	"log/slog"
	"time"
)

type RetryService struct {
	key   string
	timer *time.Timer
}

func NewRetryService(key string) *RetryService {
	return &RetryService{
		key: key,
	}
}

func (r *RetryService) retry(f func() error, retryInterval time.Duration, n uint32) {
	if n == 0 {
		slog.Warn("Retry failed for", "key", r.key)
		return
	}
	r.clearTimer()
	err := f()
	if err == nil {
		return
	}
	slog.Info("Retry scheduled", "key", r.key, "retryInterval", retryInterval)
	r.timer = time.AfterFunc(retryInterval, func() {
		r.retry(f, retryInterval, n-1)
	})
	return
}

func (r *RetryService) clearTimer() {
	if r.timer != nil {
		r.timer.Stop()
		if r.timer.C != nil {
			<-r.timer.C
		}
	}
}
