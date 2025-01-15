package dto

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// RestoreJobStatus represents a restore job status.
// @Description RestoreJobStatus represents a restore job status.
type RestoreJobStatus struct {
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

	CurrentRestore *RunningJob `yaml:"current-restore,omitempty" json:"current-job,omitempty"`
	Status         string      `yaml:"status,omitempty" json:"status,omitempty"`
	Error          string      `yaml:"error,omitempty" json:"error,omitempty"`
	Started        time.Time   `yaml:"started" json:"started"`
	Finished       *time.Time  `yaml:"finished,omitempty" json:"finished,omitempty"`
}

func NewResultFromModel(m *model.RestoreJobStatus) *RestoreJobStatus {
	if m == nil {
		return nil
	}

	r := &RestoreJobStatus{}
	r.fromModel(m)
	return r
}

func (r *RestoreJobStatus) fromModel(m *model.RestoreJobStatus) {
	r.ReadRecords = m.ReadRecords
	r.TotalBytes = m.TotalBytes
	r.ExpiredRecords = m.ExpiredRecords
	r.SkippedRecords = m.SkippedRecords
	r.IgnoredRecords = m.IgnoredRecords
	r.InsertedRecords = m.InsertedRecords
	r.ExistedRecords = m.ExistedRecords
	r.FresherRecords = m.FresherRecords
	r.IndexCount = m.IndexCount
	r.UDFCount = m.UDFCount
	r.Status = string(m.Status)
	r.Error = m.Error
	r.CurrentRestore = NewRunningJobFromModel(m.CurrentRestore)
	r.Started = m.Started
	r.Finished = m.Finished
}
