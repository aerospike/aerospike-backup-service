package model

// BackupCommonConfig represents service-level backup settings.
type BackupCommonConfig struct {
	// TimestampFormat for human-readable dates in path.
	TimestampFormat *TimestampFormat
	// ScheduleTimezone is the default timezone for evaluating backup cron
	// expressions (UTC, Local, or an IANA name). Empty means UTC.
	ScheduleTimezone string
}
