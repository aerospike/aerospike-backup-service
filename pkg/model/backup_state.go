package model

import (
	"encoding/json"
	"time"
)

// BackupState represents the state of a backup.
type BackupState struct {
	LastRun     time.Time `json:"last_run,omitempty"`
	LastIncrRun time.Time `json:"last_incr_run,omitempty"`
	Performed   int       `json:"performed,omitempty"`
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
