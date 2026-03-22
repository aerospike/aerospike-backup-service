package model

import "time"

// BackupType identifies a scheduled backup job kind (full vs incremental).
type BackupType string

const (
	BackupJobTypeFull        BackupType = "full"
	BackupJobTypeIncremental BackupType = "incremental"
)

// String returns a short human-readable label for metrics and logs.
func (j BackupType) String() string {
	if j == BackupJobTypeFull {
		return "Full backup"
	}

	return "Incremental backup"
}

// BackupRunSpec captures run-specific backup parameters passed across layers.
type BackupRunSpec struct {
	// Type is full or incremental for this run.
	Type BackupType
	// StartTime is the logical start instant for this run (paths and metadata).
	StartTime time.Time
	// TimeBounds constrains what data is included (incremental from/to, sealed to-time).
	TimeBounds TimeBounds
}
