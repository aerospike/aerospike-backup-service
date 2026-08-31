package model

import "time"

// BackupCommonConfig represents service-level backup settings.
type BackupCommonConfig struct {
	// TimestampFormat for human-readable dates in path.
	TimestampFormat *TimestampFormat
	// Timezone is the resolved default timezone for evaluating backup cron
	// expressions. Not nil after DTO conversion.
	Timezone *time.Location
	// ConfiguredTimezone is the service-level schedule-timezone as configured.
	// A blank value uses UTC.
	ConfiguredTimezone string
}

// GetTimezoneOrDefault returns the resolved schedule timezone, defaulting to UTC.
func (c *BackupCommonConfig) GetTimezoneOrDefault() *time.Location {
	if c == nil || c.Timezone == nil {
		return time.UTC
	}

	return c.Timezone
}
