package model

import (
	"time"

	"github.com/aerospike/backup-go/models"
)

type XDRConfig struct {
	// Local address, where source cluster will send data.
	LocalHost string
	// PortRange limits the range of ports that the TCP server for XDR will listen on (optional).
	PortRange *PortRange
	// Rewind is used to ship all existing records of a namespace.
	// When rewinding a namespace, XDR will scan through the index and ship
	// all the records for that namespace, partition by partition.
	// Can be `all` or number of seconds.
	Rewind string
	// Results queue size.
	// Used by TCP server for XDR.
	ResultQueueSize *int
	// Ack messages queue size.
	// Used by TCP server for XDR.
	AckQueueSize *int
	// Max number of allowed simultaneous connection to server.
	// Used by TCP server for XDR.
	MaxConns *int
	// Timeout in milliseconds for TCP read operations.
	// Used by TCP server for XDR.
	ReadTimeout *time.Duration
	// Timeout in milliseconds for TCP writes operations.
	// Used by TCP server for XDR.
	WriteTimeout *time.Duration
	// Timeout for starting TCP server for XDR.
	// If the TCP server for XDR does not receive any data within this timeout period, it will shut down.
	// This situation can occur if the LocalAddress and LocalPort options are misconfigured.
	StartTimeout *time.Duration
	// How often a backup client will send info commands to check aerospike cluster stats.
	// To measure recovery state and lag.
	PollingPeriod *time.Duration
	// Retry policy for info commands.
	InfoRetryPolicy *models.RetryPolicy
}

// GetMaxConnsOrDefault returns MaxConns value or default.
func (c *XDRConfig) GetMaxConnsOrDefault() int {
	if c.MaxConns != nil {
		return *c.MaxConns
	}
	return *defaultConfig.xdrConfig.MaxConns
}

// GetReadTimeoutOrDefault returns ReadTimeout value or default.
func (c *XDRConfig) GetReadTimeoutOrDefault() time.Duration {
	if c.ReadTimeout != nil {
		return *c.ReadTimeout
	}
	return *defaultConfig.xdrConfig.ReadTimeout
}

// GetWriteTimeoutOrDefault returns WriteTimeout value or default.
func (c *XDRConfig) GetWriteTimeoutOrDefault() time.Duration {
	if c.WriteTimeout != nil {
		return *c.WriteTimeout
	}
	return *defaultConfig.xdrConfig.WriteTimeout
}

// GetStartTimeoutOrDefault returns StartTimeout value or default.
func (c *XDRConfig) GetStartTimeoutOrDefault() time.Duration {
	if c.StartTimeout != nil {
		return *c.StartTimeout
	}
	return *defaultConfig.xdrConfig.StartTimeout
}

// GetPollingPeriodOrDefault returns PollingPeriod value or default.
func (c *XDRConfig) GetPollingPeriodOrDefault() time.Duration {
	if c.PollingPeriod != nil {
		return *c.PollingPeriod
	}
	return *defaultConfig.xdrConfig.PollingPeriod
}

// GetResultQueueSizeOrDefault returns ResultQueueSize value or default.
func (c *XDRConfig) GetResultQueueSizeOrDefault() int {
	if c.ResultQueueSize != nil {
		return *c.ResultQueueSize
	}
	return *defaultConfig.xdrConfig.ResultQueueSize
}

// GetAckQueueSizeOrDefault returns AckQueueSize value or default.
func (c *XDRConfig) GetAckQueueSizeOrDefault() int {
	if c.AckQueueSize != nil {
		return *c.AckQueueSize
	}
	return *defaultConfig.xdrConfig.AckQueueSize
}

// GetInfoRetryPolicyOrDefault returns InfoRetryPolicy value or default.
func (c *XDRConfig) GetInfoRetryPolicyOrDefault() *models.RetryPolicy {
	if c.InfoRetryPolicy != nil {
		return c.InfoRetryPolicy
	}
	return defaultConfig.xdrConfig.InfoRetryPolicy
}
