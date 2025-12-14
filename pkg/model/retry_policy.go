package model

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/optional"
	"github.com/aerospike/backup-go/models"
)

// RetryPolicy defines the configuration for retry attempts in case of failures.
type RetryPolicy struct {
	// BaseTimeout is the initial delay to wait before the first retry attempt.
	// Uses Optional[time.Duration] to distinguish three states:
	// 1. Not set: Use a system default duration.
	// 2. Set to 0: Explicitly request an immediate first retry.
	// 3. Set to T > 0: Explicitly request a delay of T.
	BaseTimeout optional.Optional[time.Duration]

	// Multiplier is used to increase the delay between subsequent retry attempts.
	// If nil, default multiplier will be used.
	// Must be >= 1.0 (for fixed or exponential backoff).
	Multiplier *float64

	// MaxRetries is the maximum number of retry attempts that will be made.
	// Uses Optional[int] to distinguish three states:
	// 1. Not set: Use a system default number.
	// 2. Set to 0: Explicitly request NO retries.
	// 3. Set to N > 0: Explicitly request N retries.
	MaxRetries optional.Optional[int]
}

// Restore Retry policy used for write operations on restore.
func (policy *RetryPolicy) Restore() *models.RetryPolicy {
	return policy.toLibraryModels(defaultConfig.restorePolicy.RetryPolicy)
}

// Backup Retry policy used for backup operation, for each NS.
func (policy *RetryPolicy) Backup() *models.RetryPolicy {
	return policy.toLibraryModels(defaultConfig.backupPolicy.RetryPolicy)
}

func (policy *RetryPolicy) toLibraryModels(defaults *RetryPolicy) *models.RetryPolicy {
	timeout := defaults.BaseTimeout.OrElse(0)
	if policy != nil && policy.BaseTimeout.Present {
		timeout = policy.BaseTimeout.Value
	}

	multiplier := *defaults.Multiplier
	if policy != nil && policy.Multiplier != nil {
		multiplier = *policy.Multiplier
	}

	maxRetries := uint(defaults.MaxRetries.OrElse(0))
	if policy != nil && policy.MaxRetries.Present {
		maxRetries = uint(policy.MaxRetries.Value)
	}

	return models.NewRetryPolicy(timeout, multiplier, maxRetries)
}
