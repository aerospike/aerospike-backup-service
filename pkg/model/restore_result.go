package model

const (
	JobStatusRunning = "RUNNING"
	JobStatusDone    = "DONE"
	JobStatusFailed  = "FAILED"
)

type RestoreResult struct {
	Number int
	Bytes  int
}

type RestoreJobStatus struct {
	RestoreResult
	Status string
	Error  error
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
