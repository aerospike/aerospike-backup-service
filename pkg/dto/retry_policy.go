package dto

import (
	"fmt"
	"time"
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
func (rp *RetryPolicy) Validate() error {
	if rp == nil {
		return nil
	}

	if rp.BaseTimeout <= 0 {
		return fmt.Errorf("BaseTimeout must be greater than 0")
	}

	if rp.Multiplier < 1 {
		return fmt.Errorf("multiplier must be greater or equal than 1")
	}

	if rp.MaxRetries < 0 {
		return fmt.Errorf("MaxRetries must be 0 or greater")
	}

	return nil
}

// GetBaseTimeout converts the BaseTimeout from milliseconds to a time.Duration.
func (rp *RetryPolicy) GetBaseTimeout() time.Duration {
	return time.Duration(rp.BaseTimeout) * time.Millisecond
}
