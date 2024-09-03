package model

import (
	"encoding/json"
	"sync"
	"time"
)

// BackupState represents the state of a backup routine.
// @Description BackupState represents the state of a backup routine.
type BackupState struct {
	sync.Mutex
	// Last time the full backup was performed.
	LastFullRun time.Time
	// Last time the incremental backup was performed.
	LastIncrRun time.Time
	// The number of successful full backups created for the routine.
	Performed int
}

// String satisfies the fmt.Stringer interface.
func (state *BackupState) String() string {
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

func (state *BackupState) LastFullRunIsEmpty() bool {
	state.Lock()
	defer state.Unlock()
	return state.LastFullRun.Equal(time.Time{})
}

func (state *BackupState) SetLastFullRun(time time.Time) {
	state.Lock()
	defer state.Unlock()
	state.LastFullRun = time
	state.Performed++
}

func (state *BackupState) SetLastIncrRun(time time.Time) {
	state.Lock()
	defer state.Unlock()
	state.LastIncrRun = time
}

func (state *BackupState) LastRun() time.Time {
	state.Lock()
	defer state.Unlock()
	if state.LastIncrRun.After(state.LastFullRun) {
		return state.LastIncrRun
	}

	return state.LastFullRun
}
