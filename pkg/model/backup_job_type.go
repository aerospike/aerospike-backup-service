package model

import "time"

// BackupJobType identifies a scheduled backup job kind (full vs incremental).
type BackupJobType string

const (
	BackupJobTypeFull        BackupJobType = "full"
	BackupJobTypeIncremental BackupJobType = "incremental"
)

// String returns a short human-readable label for metrics and logs.
func (j BackupJobType) String() string {
	if j == BackupJobTypeFull {
		return "Full backup"
	}

	return "Incremental backup"
}

// BackupRunSpec captures run-specific backup parameters passed across layers.
type BackupRunSpec struct {
	// Type is full or incremental for this run.
	Type BackupJobType
	// StartTime is the logical start instant for this run (paths and metadata).
	StartTime time.Time
	// TimeBounds constrains what data is included (incremental from/to, sealed to-time).
	TimeBounds TimeBounds
}
