package model

import (
	"time"

	"github.com/aerospike/backup-go/models"
)

// RetryPolicy defines the configuration for retry attempts in case of failures.
type RetryPolicy struct {
	// BaseTimeout is the initial delay between retry attempts.
	BaseTimeout *time.Duration

	// Multiplier is used to increase the delay between subsequent retry attempts.
	// The actual delay is calculated as: BaseTimeout * (Multiplier ^ attemptNumber)
	Multiplier *float64

	// MaxRetries is the maximum number of retry attempts that will be made.
	// If set to 0, no retries will be performed.
	MaxRetries *int
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
	timeout := *defaults.BaseTimeout
	if policy != nil && policy.BaseTimeout != nil {
		timeout = *policy.BaseTimeout
	}

	multiplier := *defaults.Multiplier
	if policy != nil && policy.Multiplier != nil {
		multiplier = *policy.Multiplier
	}

	maxRetries := uint(*defaults.MaxRetries)
	if policy != nil && policy.MaxRetries != nil {
		maxRetries = uint(*policy.MaxRetries)
	}

	return models.NewRetryPolicy(timeout, multiplier, maxRetries)
}
