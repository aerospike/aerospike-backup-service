package model

import (
	"fmt"
	"sync"
	"time"
)

// BackupTime stores execution timestamps for both full and incremental backups.
type BackupTime struct {
	mu sync.RWMutex
	// Last time the Full backup was performed.
	full *time.Time
	// Last time the Incremental backup was performed.
	incremental *time.Time
}

func NewBackupTime(lastFullBackup time.Time, lastIncrBackup time.Time) *BackupTime {
	return &BackupTime{
		full:        &lastFullBackup,
		incremental: &lastIncrBackup,
	}
}

func NewFullBackupTime(lastFullBackup time.Time) *BackupTime {
	return &BackupTime{
		full: &lastFullBackup,
	}
}

func NewNoBackupTime() *BackupTime {
	return &BackupTime{}
}

func (r *BackupTime) NoFullBackup() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.full == nil
}

func (r *BackupTime) LatestRun() *time.Time {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.full == nil {
		return copyTimePtr(r.incremental)
	}
	if r.incremental == nil {
		return copyTimePtr(r.full)
	}
	if r.incremental.After(*r.full) {
		return copyTimePtr(r.incremental)
	}
	return copyTimePtr(r.full)
}

func (r *BackupTime) SetFullBackupTime(t *time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.full = copyTimePtr(t)
}

func (r *BackupTime) SetIncrementalBackupTime(t *time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.incremental = copyTimePtr(t)
}

func (r *BackupTime) FullBackupTime() *time.Time {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return copyTimePtr(r.full)
}

func (r *BackupTime) IncrementalBackupTime() *time.Time {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return copyTimePtr(r.incremental)
}

func (r *BackupTime) String() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	full := "never"
	if r.full != nil {
		full = r.full.Format(time.RFC3339)
	}

	incremental := "never"
	if r.incremental != nil {
		incremental = r.incremental.Format(time.RFC3339)
	}

	return fmt.Sprintf("Full: %s, Incremental: %s", full, incremental)
}

func copyTimePtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	copied := *t
	return &copied
}
