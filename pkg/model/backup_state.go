package model

import (
	"encoding/json"
	"time"
)

// BackupState represents the state of a backup routine.
// @Description BackupState represents the state of a backup routine.
type BackupState struct {
	// Last time the full backup was performed.
	LastFullRun time.Time `yaml:"last-run,omitempty" json:"last-run,omitempty"`
	// Last time the incremental backup was performed.
	LastIncrRun time.Time `yaml:"last-incr-run,omitempty" json:"last-incr-run,omitempty"`
	// The number of successful full backups created for the routine.
	Performed int `yaml:"performed,omitempty" json:"performed,omitempty"`
}

// String satisfies the fmt.Stringer interface.
func (state BackupState) String() string {
	backupState, err := json.Marshal(state)
	if err != nil {
		return err.Error()
	}
	return string(backupState)
}

// NewBackupState returns a BackupState with the default values.
func NewBackupState() *BackupState {
	return &BackupState{}
}
