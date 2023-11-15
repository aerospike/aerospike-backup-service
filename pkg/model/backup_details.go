package model

import (
	"encoding/json"
	"time"
)

// BackupDetails contains information about a backup.
type BackupDetails struct {
	Key          *string    `yaml:"key,omitempty" json:"key,omitempty"`
	LastModified *time.Time `yaml:"last-modified,omitempty" json:"last-modified,omitempty"`
	Size         *int64     `yaml:"size,omitempty" json:"size,omitempty"`
}

// String satisfies the fmt.Stringer interface.
func (details BackupDetails) String() string {
	backupDetails, err := json.Marshal(details)
	if err != nil {
		return err.Error()
	}
	return string(backupDetails)
}
