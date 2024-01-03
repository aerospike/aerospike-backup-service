package model

type Status string

const (
	JobStatusRunning Status = "Running"
	JobStatusDone    Status = "Done"
	JobStatusFailed  Status = "Failed"
)

type RestoreJobStatus struct {
	RestoreResult
	Status Status `yaml:"status,omitempty" json:"status,omitempty" enums:"Running,Done,Failed"`
	Error  error  `yaml:"error,omitempty" json:"error,omitempty"`
}

type RestoreResult struct {
	Number int `yaml:"total-records,omitempty" json:"total-records,omitempty"`
	Bytes  int `yaml:"total-bytes,omitempty" json:"total-bytes,omitempty"`
}

func NewRestoreResult() *RestoreResult {
	return &RestoreResult{}
}

func NewRestoreJobStatus() *RestoreJobStatus {
	return &RestoreJobStatus{
		Status: JobStatusRunning,
	}
}

func (r RestoreJobStatus) IsSuccess() bool {
	return r.Status == JobStatusFailed
}
