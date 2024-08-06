package model

type JobStatus string

const (
	JobStatusRunning JobStatus = "Running"
	JobStatusDone    JobStatus = "Done"
	JobStatusFailed  JobStatus = "Failed"
)

// RestoreJobStatus represents a restore job status.
// @Description RestoreJobStatus represents a restore job status.
type RestoreJobStatus struct {
	RestoreStats
	CurrentRestore *RunningJob
	Status         JobStatus `yaml:"status,omitempty" json:"status,omitempty" enums:"Running,Done,Failed"`
	Error          string    `yaml:"error,omitempty" json:"error,omitempty"`
}

type RestoreStats struct {
	ReadRecords     uint64 `yaml:"read-records,omitempty" json:"read-records,omitempty" format:"int64" example:"10"`
	TotalBytes      uint64 `yaml:"total-bytes,omitempty" json:"total-bytes,omitempty" format:"int64" example:"2000"`
	ExpiredRecords  uint64 `yaml:"expired-records,omitempty" json:"expired-records,omitempty" format:"int64" example:"2"`
	SkippedRecords  uint64 `yaml:"skipped-records,omitempty" json:"skipped-records,omitempty" format:"int64" example:"4"`
	IgnoredRecords  uint64 `yaml:"ignored-records,omitempty" json:"ignored-records,omitempty" format:"int64" example:"12"`
	InsertedRecords uint64 `yaml:"inserted-records,omitempty" json:"inserted-records,omitempty" format:"int64" example:"8"`
	ExistedRecords  uint64 `yaml:"existed-records,omitempty" json:"existed-records,omitempty" format:"int64" example:"15"`
	FresherRecords  uint64 `yaml:"fresher-records,omitempty" json:"fresher-records,omitempty" format:"int64" example:"5"`
	IndexCount      uint64 `yaml:"index-count,omitempty" json:"index-count,omitempty" format:"int64" example:"3"`
	UDFCount        uint64 `yaml:"udf-count,omitempty" json:"udf-count,omitempty" format:"int64" example:"1"`
}
