package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

type Executor interface {
	run(label string, f func() error) error
}

type SimpleExecutor struct{}

func (e *SimpleExecutor) run(_ string, f func() error) error {
	return f()
}

// RetryExecutor is a service for retrying a function with a specified interval
// and a maximum number of attempts.
type RetryExecutor struct {
	logger        *slog.Logger
	retryInterval time.Duration
	maxAttempts   int
}

// NewRetryExecutor returns a new RetryExecutor instance.
//   - retryInterval is the interval between retry attempts
//   - maxAttempts is the maximum number of retry attempts
//   - logger is used for logging retry attempts and errors
func NewRetryExecutor(retryInterval time.Duration, maxAttempts int, logger *slog.Logger) Executor {
	return &RetryExecutor{
		logger:        logger,
		retryInterval: retryInterval,
		maxAttempts:   maxAttempts,
	}
}

// retry attempts to execute the given function up to maxAttempts with the specified retryInterval.
// If all attempts fail, it returns an error.
func (r *RetryExecutor) run(label string, f func() error) error {
	var lastErr error
	for attempt := 1; attempt <= r.maxAttempts; attempt++ {
		lastErr = f()

		if lastErr == nil || errors.Is(lastErr, context.Canceled) {
			return nil // success
		}

		if attempt < r.maxAttempts { // Log and wait only if there are attempts left
			r.logger.Info("Execution failed, retrying...",
				slog.String("label", label),
				slog.Int("attempt", attempt),
				slog.Int("maxAttempts", r.maxAttempts),
				slog.Any("retryInterval", r.retryInterval),
				slog.Any("err", lastErr))
			time.Sleep(r.retryInterval) // wait before the next attempt
		}
	}

	// If we exhausted all attempts, return an error
	return fmt.Errorf("%s failed after %d attempts: %w", label, r.maxAttempts, lastErr)
}
