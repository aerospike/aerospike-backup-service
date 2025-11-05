package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
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
	logger       *slog.Logger
	policy       models.RetryPolicy
	nonRetryable []error
}

// newRetryExecutor returns a new retryExecutor instance.
// nonRetryable is an optional list of errors that should not be retried.
func newRetryExecutor(policy models.RetryPolicy, logger *slog.Logger, nonRetryable ...error) executor {
	return &retryExecutor{
		logger:       logger,
		policy:       policy,
		nonRetryable: nonRetryable,
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
			return lastErr // success or canceled
		}

		// If error is configured as non-retryable, abort immediately
		for _, nre := range r.nonRetryable {
			if nre != nil && errors.Is(lastErr, nre) {
				r.logger.Info("Non-retryable error encountered, aborting without retry",
					slog.String("label", label),
					attr.Error(lastErr))
				return lastErr
			}
		}

		if attempt < totalAttempts { // Log and wait only if there are attempts left
			onRetry()
			r.logger.Info("Execution failed, retrying...",
				slog.String("label", label),
				slog.Any("attempt", attempt),
				slog.Any("maxAttempts", r.policy.MaxRetries),
				slog.Any("retryInterval", retryInterval),
				attr.Error(lastErr))
			time.Sleep(retryInterval) // wait before the next attempt
			retryInterval = time.Duration(float64(retryInterval) * r.policy.Multiplier)
		}
	}

	// If we exhausted all attempts, return an error
	return fmt.Errorf("%s failed after %d attempts: %w", label, totalAttempts, lastErr)
}
