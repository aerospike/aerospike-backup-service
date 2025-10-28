package model

// BackupCommonConfig represents service-level backup settings.
type BackupCommonConfig struct {
	// TimestampFormat for human-readable dates in path.
	TimestampFormat *TimestampFormat
}
