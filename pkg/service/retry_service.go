package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/backup-go/models"
)

// executor defines an interface for executing functions with retries.
// label defines a job, it is only used in logs and error messages.
// onRetry is called for side effects when a retry occurs.
type executor interface {
	run(label string, f func() error, onRetry func()) error
}

type simpleExecutor struct{}

func (e *simpleExecutor) run(_ string, f func() error, _ func()) error {
	return f()
}

// retryExecutor is a service for retrying a function with a specified interval
// and a maximum number of attempts.
type retryExecutor struct {
	logger *slog.Logger
	policy models.RetryPolicy
}

// newRetryExecutor returns a new retryExecutor instance.
func newRetryExecutor(policy models.RetryPolicy, logger *slog.Logger) executor {
	return &retryExecutor{
		logger: logger,
		policy: policy,
	}
}

// retry attempts to execute the given function up to maxAttempts with the specified retryInterval.
// If all attempts fail, it returns an error.
func (r *retryExecutor) run(label string, f func() error, onRetry func()) error {
	var (
		lastErr       error
		retryInterval = r.policy.BaseTimeout
		totalAttempts = r.policy.MaxRetries + 1
	)

	for attempt := uint(1); attempt <= totalAttempts; attempt++ {
		lastErr = f()
		if lastErr == nil || errors.Is(lastErr, context.Canceled) {
			return lastErr // success
		}

		if attempt < r.policy.MaxRetries { // Log and wait only if there are attempts left
			onRetry()
			r.logger.Info("Execution failed, retrying...",
				slog.String("label", label),
				slog.Any("attempt", attempt),
				slog.Any("maxAttempts", r.policy.MaxRetries),
				slog.Any("retryInterval", retryInterval),
				slog.Any("err", lastErr))
			time.Sleep(retryInterval) // wait before the next attempt
			retryInterval = time.Duration(float64(retryInterval) * r.policy.Multiplier)
		}
	}

	// If we exhausted all attempts, return an error
	return fmt.Errorf("%s failed after %d attempts: %w", label, totalAttempts, lastErr)
}
