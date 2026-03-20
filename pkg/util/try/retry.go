package try

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/backup-go/models"
	"github.com/aerospike/backup-go/pkg/asinfo"
)

var nonRetryableErrors = []error{asinfo.ErrNoNode}

// Retry runs f with retries according to policy.
// Pass a logger scoped with context (e.g. logger.With(slog.String("label", "backup"))).
// onRetry is invoked before each Retry (not on the final failed attempt).
func Retry(policy models.RetryPolicy, logger *slog.Logger, f func() error, onRetry func()) error {
	var (
		lastErr       error
		retryInterval = policy.BaseTimeout
		totalAttempts = policy.MaxRetries + 1
	)

	for attempt := uint(1); attempt <= totalAttempts; attempt++ {
		lastErr = f()
		if lastErr == nil || errors.Is(lastErr, context.Canceled) {
			return lastErr
		}

		for _, nre := range nonRetryableErrors {
			if nre != nil && errors.Is(lastErr, nre) {
				logger.Info("Non-retryable error encountered, aborting without Retry", attr.Error(lastErr))
				return lastErr
			}
		}

		if attempt < totalAttempts {
			onRetry()
			logger.Info("Execution failed, retrying...",
				slog.Any("attempt", attempt),
				slog.Any("maxAttempts", policy.MaxRetries),
				slog.Any("retryInterval", retryInterval),
				attr.Error(lastErr))
			time.Sleep(retryInterval)
			retryInterval = time.Duration(float64(retryInterval) * policy.Multiplier)
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", totalAttempts, lastErr)
}
