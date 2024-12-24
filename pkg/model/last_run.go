package model

import "time"

// LastBackupRun stores the last run times for both full and incremental backups.
type LastBackupRun struct {
	// Last time the Full backup was performed.
	Full *time.Time
	// Last time the Incremental backup was performed.
	Incremental *time.Time
}

func NewLastRun(lastFullBackup *time.Time, lastIncrBackup *time.Time) LastBackupRun {
	return LastBackupRun{
		Full:        lastFullBackup,
		Incremental: lastIncrBackup,
	}
}

func (r *LastBackupRun) NoFullBackup() bool {
	return r.Full == nil
}

func (r *LastBackupRun) LastAnyRun() *time.Time {
	if r.Incremental != nil && r.Full != nil && r.Incremental.After(*r.Full) {
		return r.Incremental
	}

	return r.Full
}
