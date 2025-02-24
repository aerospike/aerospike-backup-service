package model

import (
	"fmt"
	"sync"
	"time"
)

// LastBackupRun stores the last run times for both full and incremental backups.
type LastBackupRun struct {
	mu sync.RWMutex
	// Last time the Full backup was performed.
	full *time.Time
	// Last time the Incremental backup was performed.
	incremental *time.Time
}

func NewLastBackupRun(lastFullBackup *time.Time, lastIncrBackup *time.Time) *LastBackupRun {
	return &LastBackupRun{
		full:        lastFullBackup,
		incremental: lastIncrBackup,
	}
}

func (r *LastBackupRun) NoFullBackup() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.full == nil
}

func (r *LastBackupRun) LatestRun() *time.Time {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.incremental != nil && r.full != nil && r.incremental.After(*r.full) {
		return r.incremental
	}
	return r.full
}

func (r *LastBackupRun) SetFullBackupTime(t *time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.full = t
}

func (r *LastBackupRun) SetIncrementalBackupTime(t *time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.incremental = t
}

func (r *LastBackupRun) FullBackupTime() *time.Time {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.full
}

func (r *LastBackupRun) IncrementalBackupTime() *time.Time {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.incremental
}

func (r *LastBackupRun) String() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	full := "nil"
	if r.full != nil {
		full = r.full.Format(time.RFC3339)
	}

	incremental := "nil"
	if r.incremental != nil {
		incremental = r.incremental.Format(time.RFC3339)
	}

	return fmt.Sprintf("Full: %s, Incremental: %s", full, incremental)
}
