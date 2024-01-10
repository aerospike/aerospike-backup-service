package model

import (
	"encoding/json"
	"time"
)

// BackupDetails contains information about a backup.
// @Description BackupDetails contains information about a backup.
type BackupDetails struct {
	// The path to the backup files.
	Key *string `yaml:"key,omitempty" json:"key,omitempty"`
	// The backup time in the ISO 8601 format.
	LastModified *time.Time `yaml:"last-modified,omitempty" json:"last-modified,omitempty"`
	// The size of the backup in bytes.
	Size *int64 `yaml:"size,omitempty" json:"size,omitempty"`
}

// String satisfies the fmt.Stringer interface.
func (details BackupDetails) String() string {
	backupDetails, err := json.Marshal(details)
	if err != nil {
		return err.Error()
	}
	return string(backupDetails)
}

// BackupMetadata internal container for backup metadata
type BackupMetadata struct {
	Created time.Time `yaml:"created,omitempty" json:"created,omitempty"`
}
