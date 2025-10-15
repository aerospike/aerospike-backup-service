package dto

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// RetryPolicy defines the configuration for retry attempts in case of failures.
// @Description RetryPolicy defines the configuration for retry attempts in case of failures.
type RetryPolicy struct {
	// BaseTimeout is the initial delay between retry attempts, in milliseconds.
	BaseTimeout *int64 `json:"base-timeout" yaml:"base-timeout"`

	// Multiplier is used to increase the delay between subsequent retry attempts.
	// The actual delay is calculated as: BaseTimeout * (Multiplier ^ attemptNumber)
	Multiplier *float64 `json:"multiplier" yaml:"multiplier"`

	// MaxRetries is the maximum number of retry attempts that will be made.
	// If set to 0, no retries will be performed.
	MaxRetries *int `json:"max-retries" yaml:"max-retries"`
}

// Validate checks if the RetryPolicy fields are valid.
func (r *RetryPolicy) Validate() error {
	if r == nil {
		return nil
	}

	if r.BaseTimeout != nil {
		if *r.BaseTimeout > (24 * time.Hour).Milliseconds() {
			return errValidationInvalidValue("base-timeout", *r.BaseTimeout, "should not exceed 24 hours")
		}

		if *r.BaseTimeout <= 0 {
			return errValidationNonPositive("base-timeout", *r.BaseTimeout)
		}
	}

	if r.Multiplier != nil && *r.Multiplier < 1 {
		return errValidationInvalidValue("multiplier", *r.Multiplier, "must be greater or equal than 1")
	}

	if r.MaxRetries != nil && *r.MaxRetries < 0 {
		return errValidationNegative("max-retries", *r.MaxRetries)
	}

	return nil
}

func (r *RetryPolicy) ToModel() *model.RetryPolicy {
	if r == nil {
		return nil
	}

	return &model.RetryPolicy{
		BaseTimeout: millisToDuration(r.BaseTimeout),
		Multiplier:  r.Multiplier,
		MaxRetries:  r.MaxRetries,
	}
}
func newRetryPolicyFromModel(m *model.RetryPolicy) *RetryPolicy {
	if m == nil {
		return nil
	}

	return &RetryPolicy{
		BaseTimeout: durationToMillis(m.BaseTimeout),
		Multiplier:  m.Multiplier,
		MaxRetries:  m.MaxRetries,
	}
}
