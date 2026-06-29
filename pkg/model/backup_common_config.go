package model

// BackupCommonConfig represents service-level backup settings.
type BackupCommonConfig struct {
	// TimestampFormat for human-readable dates in path.
	TimestampFormat *TimestampFormat
	// BackupMode selects scan or server backup for the entire service instance.
	BackupMode *BackupMode
}

// GetBackupModeOrDefault returns the configured backup mode, defaulting to scan.
func (c *BackupCommonConfig) GetBackupModeOrDefault() BackupMode {
	if c != nil && c.BackupMode != nil {
		return *c.BackupMode
	}

	return BackupModeScan
}
