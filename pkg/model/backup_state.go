package model

import (
	"encoding/json"
	"time"
)

// BackupState represents the state of a backup.
type BackupState struct {
	LastRun     time.Time `yaml:"last-run,omitempty" json:"last-run,omitempty"`
	LastIncrRun time.Time `yaml:"last-incr-run,omitempty" json:"last-incr-run,omitempty"`
	Performed   int       `yaml:"performed,omitempty" json:"performed,omitempty"`
}

// String satisfies the fmt.Stringer interface.
func (state BackupState) String() string {
	backupState, err := json.Marshal(state)
	if err != nil {
		return err.Error()
	}
	return string(backupState)
}

// NewBackupState returns a BackupState with default values.
func NewBackupState() *BackupState {
	return &BackupState{}
}
