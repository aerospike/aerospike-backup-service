package dto

import (
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// XDRConfig represents the configuration for XDR backups.
// @Description XDRConfig represents the configuration for XDR backups.
type XDRConfig struct {
	// LocalHost is the local address where the source cluster will send data.
	LocalHost string `yaml:"local-host,omitempty" json:"local-host,omitempty" example:"127.0.0.1"`

	// PortRange limits the range of ports that the TCP server for XDR will listen on (optional).
	PortRange *PortRange `yaml:"port-range,omitempty" json:"port-range,omitempty"`

	// ResultQueueSize is the size of the results queue used by the TCP server for XDR.
	ResultQueueSize *int `yaml:"result-queue-size,omitempty" json:"result-queue-size,omitempty" example:"1000"`

	// AckQueueSize is the size of the acknowledgment queue used by the TCP server for XDR.
	AckQueueSize *int `yaml:"ack-queue-size,omitempty" json:"ack-queue-size,omitempty" example:"100"`

	// MaxConns specifies the maximum number of allowed simultaneous connections to the server.
	// Used by the TCP server for XDR.
	MaxConns *int `yaml:"max-conns,omitempty" json:"max-conns,omitempty" example:"100"`

	// ReadTimeout is the timeout in milliseconds for TCP read operations.
	// Used by the TCP server for XDR.
	ReadTimeout *int64 `yaml:"read-timeout,omitempty" json:"read-timeout,omitempty" example:"5000"`

	// WriteTimeout is the timeout in milliseconds for TCP write operations.
	// Used by the TCP server for XDR.
	WriteTimeout *int64 `yaml:"write-timeout,omitempty" json:"write-timeout,omitempty" example:"5000"`

	// StartTimeout is the timeout for starting the TCP server for XDR.
	// If the TCP server does not receive any data within this timeout period, it will shut down.
	// This situation can occur if the LocalAddress and LocalPort options are misconfigured.
	StartTimeout *int64 `yaml:"start-timeout,omitempty" json:"start-timeout,omitempty" example:"3000"`

	// PollingPeriod specifies how often a backup client will send info commands to check Aerospike cluster stats.
	// Used to measure recovery state and lag.
	PollingPeriod *int64 `yaml:"polling-period,omitempty" json:"polling-period,omitempty" example:"60000"`

	// InfoRetryPolicy defines the retry policy for info commands.
	InfoRetryPolicy *RetryPolicy `yaml:"info-retry-policy,omitempty" json:"info-retry-policy,omitempty"`
}

func (x *XDRConfig) Validate() error {
	if x == nil {
		return nil
	}

	if x.LocalHost == "" {
		return errValidationEmptyField("local-host")
	}

	if err := x.PortRange.Validate(); err != nil {
		return fmt.Errorf("invalid port range: %w", err)
	}

	if x.ResultQueueSize != nil && *x.ResultQueueSize <= 0 {
		return errValidationNonPositive("result-queue-size", *x.ResultQueueSize)
	}

	if x.AckQueueSize != nil && *x.AckQueueSize <= 0 {
		return errValidationNonPositive("ack-queue-size", *x.AckQueueSize)
	}

	if x.MaxConns != nil && *x.MaxConns <= 0 {
		return errValidationNonPositive("max-conns", *x.MaxConns)
	}

	if x.ReadTimeout != nil && *x.ReadTimeout < 0 {
		return errValidationNegative("read-timeout", *x.ReadTimeout)
	}

	if x.WriteTimeout != nil && *x.WriteTimeout < 0 {
		return errValidationNegative("write-timeout", *x.WriteTimeout)
	}

	if x.StartTimeout != nil && *x.StartTimeout < 0 {
		return errValidationNegative("start-timeout", *x.StartTimeout)
	}

	if x.PollingPeriod != nil && *x.PollingPeriod < 0 {
		return errValidationNegative("polling-period", *x.PollingPeriod)
	}

	if err := x.InfoRetryPolicy.Validate(); err != nil {
		return err
	}

	return nil
}

func (x *XDRConfig) ToModel() *model.XDRConfig {
	if x == nil {
		return nil
	}

	return &model.XDRConfig{
		LocalHost:       x.LocalHost,
		ResultQueueSize: x.ResultQueueSize,
		AckQueueSize:    x.AckQueueSize,
		MaxConns:        x.MaxConns,
		ReadTimeout:     x.ReadTimeout,
		WriteTimeout:    x.WriteTimeout,
		StartTimeout:    x.StartTimeout,
		PollingPeriod:   x.PollingPeriod,
		InfoRetryPolicy: x.InfoRetryPolicy.ToModel(),
	}
}

func newXDRConfigFromModel(m *model.XDRConfig) *XDRConfig {
	if m == nil {
		return nil
	}

	return &XDRConfig{
		LocalHost:       m.LocalHost,
		ResultQueueSize: m.ResultQueueSize,
		AckQueueSize:    m.AckQueueSize,
		MaxConns:        m.MaxConns,
		ReadTimeout:     m.ReadTimeout,
		WriteTimeout:    m.WriteTimeout,
		StartTimeout:    m.StartTimeout,
		PollingPeriod:   m.PollingPeriod,
		InfoRetryPolicy: newRetryPolicyFromModel(m.InfoRetryPolicy),
	}
}
