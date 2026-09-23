package model

import "time"

// BackupType identifies a scheduled backup job kind (full vs incremental).
type BackupType string

const (
	BackupTypeFull        BackupType = "full"
	BackupTypeIncremental BackupType = "incremental"
)

// BackupRunSpec captures run-specific backup parameters passed across layers.
type BackupRunSpec struct {
	// Type is full or incremental for this run.
	Type BackupType
	// StartTime is the logical start instant for this run (paths and metadata).
	StartTime time.Time
	// TimeBounds constrains what data is included (incremental from/to, sealed to-time).
	TimeBounds TimeBounds
}

// NamespaceRun is the backup of one namespace within a routine run.
type NamespaceRun struct {
	// Routine is the routine being backed up.
	Routine *BackupRoutine
	// Namespace is the namespace this backup covers.
	Namespace string
	// Spec is the routine run this backup belongs to.
	Spec BackupRunSpec
}
