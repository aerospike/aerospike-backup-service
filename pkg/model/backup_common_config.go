package model

// BackupCommonConfig represents service-level backup settings.
type BackupCommonConfig struct {
	// TimestampFormat for human-readable dates in path.
	TimestampFormat *TimestampFormat
	// Timezone is the service-level schedule timezone.
	Timezone Location
	// BackupMode selects scan or server backup for the whole service instance.
	BackupMode BackupMode
}

// GetBackupModeOrDefault returns the configured backup mode, or scan when none is set.
func (c *BackupCommonConfig) GetBackupModeOrDefault() BackupMode {
	if c.BackupMode == "" {
		return BackupModeScan
	}

	return c.BackupMode
}
