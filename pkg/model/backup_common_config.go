package model

import "time"

// BackupCommonConfig represents service-level backup settings.
type BackupCommonConfig struct {
	// TimestampFormat for human-readable dates in path.
	TimestampFormat *TimestampFormat
	// ScheduleTimezone is the default timezone for evaluating backup cron
	// expressions (UTC, Local, or an IANA name). Empty means UTC.
	ScheduleTimezone string
}

// GetTimezoneOrDefault returns the parsed schedule timezone, defaulting to UTC.
// Invalid values are rejected during DTO validation.
func (c *BackupCommonConfig) GetTimezoneOrDefault() *time.Location {
	if c == nil {
		return time.UTC
	}

	timezone, err := ParseScheduleTimezone(c.ScheduleTimezone)
	if err != nil {
		return time.UTC
	}

	return timezone
}
