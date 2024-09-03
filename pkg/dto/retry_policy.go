package dto

import (
	"fmt"
	"time"

	"github.com/aerospike/backup-go/models"
)

// RetryPolicy defines the configuration for retry attempts in case of failures.
// @Description RetryPolicy defines the configuration for retry attempts in case of failures.
type RetryPolicy struct {
	// BaseTimeout is the initial delay between retry attempts, in milliseconds.
	BaseTimeout int64 `json:"baseTimeout" yaml:"baseTimeout"`

	// Multiplier is used to increase the delay between subsequent retry attempts.
	// The actual delay is calculated as: BaseTimeout * (Multiplier ^ attemptNumber)
	Multiplier float64 `json:"multiplier" yaml:"multiplier"`

	// MaxRetries is the maximum number of retry attempts that will be made.
	// If set to 0, no retries will be performed.
	MaxRetries int `json:"maxRetries" yaml:"maxRetries"`
}

// Validate checks if the RetryPolicy fields are valid.
func (r *RetryPolicy) Validate() error {
	if r == nil {
		return nil
	}

	if r.BaseTimeout <= 0 {
		return fmt.Errorf("BaseTimeout must be greater than 0")
	}

	if r.Multiplier < 1 {
		return fmt.Errorf("multiplier must be greater or equal than 1")
	}

	if r.MaxRetries < 0 {
		return fmt.Errorf("MaxRetries must be 0 or greater")
	}

	return nil
}

// GetBaseTimeout converts the BaseTimeout from milliseconds to a time.Duration.
func (r *RetryPolicy) GetBaseTimeout() time.Duration {
	return time.Duration(r.BaseTimeout) * time.Millisecond
}

func (r *RetryPolicy) ToModel() *models.RetryPolicy {
	if r == nil {
		return nil
	}

	return &models.RetryPolicy{
		BaseTimeout: r.GetBaseTimeout(),
		Multiplier:  r.Multiplier,
		MaxRetries:  uint(r.MaxRetries),
	}
}
