package model

type JobStatus string

const (
	JobStatusRunning   JobStatus = "Running"
	JobStatusDone      JobStatus = "Done"
	JobStatusFailed    JobStatus = "Failed"
	JobStatusCancelled JobStatus = "Cancelled"
)

// RestoreJobStatus represents a restore job status.
type RestoreJobStatus struct {
	ReadRecords     uint64
	TotalBytes      uint64
	ExpiredRecords  uint64
	SkippedRecords  uint64
	IgnoredRecords  uint64
	InsertedRecords uint64
	ExistedRecords  uint64
	FresherRecords  uint64
	IndexCount      uint64
	UDFCount        uint64

	CurrentRestore *RunningJob
	Status         JobStatus
	Error          string
}
