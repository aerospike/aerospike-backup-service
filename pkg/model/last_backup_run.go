package model

import (
	"sync"
	"time"
)

// LastBackupRun stores the last run times for both full and incremental backups.
type LastBackupRun struct {
	mu          sync.RWMutex
	Full        *time.Time
	Incremental *time.Time
}

func NewLastRun(lastFullBackup *time.Time, lastIncrBackup *time.Time) *LastBackupRun {
	return &LastBackupRun{
		Full:        lastFullBackup,
		Incremental: lastIncrBackup,
	}
}

func (r *LastBackupRun) NoFullBackup() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Full == nil
}

func (r *LastBackupRun) LatestRun() *time.Time {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.Incremental != nil && r.Full != nil && r.Incremental.After(*r.Full) {
		return r.Incremental
	}
	return r.Full
}

func (r *LastBackupRun) SetFullBackupTime(t *time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Full = t
}

func (r *LastBackupRun) SetIncrementalBackupTime(t *time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Incremental = t
}
