package model

import "time"

// ServerBackupJob is a server-side backup job of one namespace, as the server last reported it.
type ServerBackupJob struct {
	JobID     string
	Namespace string
	// State is the server's job state, e.g. COMPLETE, FAILED or ABORTED.
	State string
	// ErrorReason is set by the server when the job failed or was aborted.
	ErrorReason string
	// AbortOrigin tells what aborted the job: operator, server or cluster.
	AbortOrigin string
	// StartTime is nil while the job has not started.
	StartTime *time.Time
	// FinishTime is nil while the job has not finished.
	FinishTime      *time.Time
	RecordsBackedUp uint64
	BytesBackedUp   uint64
	// RecordsReadFailed counts the records the server could not read. When it is above zero the
	// backup lacks those records, even if the job's state is COMPLETE.
	RecordsReadFailed uint64
}
