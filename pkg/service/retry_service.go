package service

import (
	"log/slog"
	"sync"
	"time"
)

// RetryService a service for retrying a function with a specified interval
// and number of attempts.
type RetryService struct {
	logger *slog.Logger
	timer  *time.Timer
	mu     sync.Mutex
}

// NewRetryService returns a new RetryService instance.
//   - logger is used for logging retry attempts and errors
func NewRetryService(logger *slog.Logger) *RetryService {
	return &RetryService{
		logger: logger,
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
		r.logger.Warn("Execution failed, no retry attempts left",
			slog.Any("err", err))
		return
	}

	r.logger.Info("Execution failed, retry scheduled",
		slog.Any("retryInterval", retryInterval),
		slog.Any("attempts", n-1),
		slog.Any("err", err))

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
