package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/aerospike/backup-go/models"
	"log/slog"
	"math"
	"time"
)

// RetryService is a service for retrying a function with a specified interval
// and a maximum number of attempts.
type RetryService struct {
	logger *slog.Logger
	policy models.RetryPolicy
}

// NewRetryService returns a new RetryService instance.
func NewRetryService(policy models.RetryPolicy, logger *slog.Logger) *RetryService {
	return &RetryService{
		logger: logger,
		policy: policy,
	}
}

// retry attempts to execute the given function up to maxAttempts with the specified retryInterval.
// If all attempts fail, it returns an error.
func (r *RetryService) retry(label string, f func() error) error {
	var lastErr error
	for attempt := uint(1); attempt <= r.policy.MaxRetries; attempt++ {
		lastErr = f()

		if lastErr == nil || errors.Is(lastErr, context.Canceled) {
			return nil // success
		}

		retryInterval := time.Duration(float64(r.policy.BaseTimeout.Milliseconds())*math.Pow(r.policy.Multiplier, float64(attempt-1))) * time.Millisecond

		if attempt < r.policy.MaxRetries { // Log and wait only if there are attempts left
			r.logger.Info("Execution failed, retrying...",
				slog.String("label", label),
				slog.Any("attempt", attempt),
				slog.Any("maxAttempts", r.policy.MaxRetries),
				slog.Any("retryInterval", retryInterval),
				slog.Any("err", lastErr))
			time.Sleep(retryInterval) // wait before the next attempt
		}
	}

	// If we exhausted all attempts, return an error
	return fmt.Errorf("%s failed after %d attempts: %w", label, r.maxAttempts, lastErr)
}
