package service

import (
	"fmt"
	"log/slog"
	"time"
)

// RetryService is a service for retrying a function with a specified interval
// and a maximum number of attempts.
type RetryService struct {
	logger        *slog.Logger
	retryInterval time.Duration
	maxAttempts   int32
}

// NewRetryService returns a new RetryService instance.
//   - retryInterval is the interval between retry attempts
//   - maxAttempts is the maximum number of retry attempts
//   - logger is used for logging retry attempts and errors
func NewRetryService(retryInterval time.Duration, maxAttempts int32, logger *slog.Logger) *RetryService {
	return &RetryService{
		logger:        logger,
		retryInterval: retryInterval,
		maxAttempts:   maxAttempts,
	}
}

// retry attempts to execute the given function up to maxAttempts with the specified retryInterval.
// If all attempts fail, it returns an error.
func (r *RetryService) retry(f func() error) error {
	var lastErr error
	for attempt := int32(1); attempt <= r.maxAttempts; attempt++ {
		lastErr = f()
		if lastErr == nil {
			return nil // success
		}

		if attempt < r.maxAttempts { // Log and wait only if there are attempts left
			r.logger.Info("Execution failed, retrying...",
				slog.Any("attempt", attempt),
				slog.Any("maxAttempts", r.maxAttempts),
				slog.Any("retryInterval", r.retryInterval),
				slog.Any("err", lastErr))
			time.Sleep(r.retryInterval) // wait before the next attempt
		}
	}

	// If we exhausted all attempts, return an error
	return fmt.Errorf("failed after %d attempts: %w", r.maxAttempts, lastErr)
}
