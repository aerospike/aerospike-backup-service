package model

// ServiceBackupConfig represents service-level backup settings.
type ServiceBackupConfig struct {
	// TimestampFormat for human-readable dates in path.
	TimestampFormat *TimestampFormat
}
