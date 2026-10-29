package dto

import (
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// BackupMode selects how the whole service instance backs up and restores data.
// @Description BackupMode selects how the whole service instance backs up and restores data.
type BackupMode string

const (
	// BackupModeScan is the default mode: the service scans the cluster and writes the data itself.
	BackupModeScan BackupMode = "scan"
	// BackupModeServer delegates backup and restore to the Aerospike server, which writes to S3 directly.
	BackupModeServer BackupMode = "server"
)

var backupModes = []BackupMode{BackupModeScan, BackupModeServer}

// Validate checks that the backup mode is supported.
func (m BackupMode) Validate() error {
	if m == "" {
		return nil
	}
	if _, ok := canonicalEnum(m, backupModes); ok {
		return nil
	}

	return errValidationInvalidValue("backup-mode", m, backupModes)
}

// ToModel converts the DTO backup mode to the model type.
func (m BackupMode) ToModel() model.BackupMode {
	if m == "" {
		return ""
	}
	c, _ := canonicalEnum(m, backupModes)
	return model.BackupMode(c)
}

// NewBackupModeFromModel creates a DTO backup mode from the model type.
func NewBackupModeFromModel(m model.BackupMode) BackupMode {
	return BackupMode(m)
}
