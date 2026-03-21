package model

import "time"

// BackupJobType identifies a scheduled backup job kind (full vs incremental).
type BackupJobType string

const (
	BackupJobTypeFull         BackupJobType = "full"
	BackupJobTypeIncremental BackupJobType = "incremental"
)

func (j BackupJobType) String() string {
	if j == BackupJobTypeFull {
		return "Full backup"
	}

	return "Incremental backup"
}

// BackupRunSpec captures run-specific backup parameters passed across layers.
type BackupRunSpec struct {
	Type       BackupJobType
	StartTime  time.Time
	TimeBounds TimeBounds
}
