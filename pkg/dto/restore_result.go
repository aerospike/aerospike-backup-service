package dto

import (
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// JobStatus represents possible states of restore jobs.
// @Description JobStatus represents possible states of restore jobs.
type JobStatus string

const (
	JobStatusRunning   JobStatus = "Running"
	JobStatusDone      JobStatus = "Done"
	JobStatusFailed    JobStatus = "Failed"
	JobStatusCancelled JobStatus = "Cancelled"
)

var allJobStatuses = []JobStatus{
	JobStatusRunning,
	JobStatusDone,
	JobStatusFailed,
	JobStatusCancelled,
}

// restoreStatusFromString returns the corresponding JobStatus enum for string representation.
func restoreStatusFromString(s string) (value JobStatus, ok bool) {
	for _, status := range allJobStatuses {
		if strings.EqualFold(s, string(status)) {
			return status, true
		}
	}

	return "", false
}

// RestoreJobStatus represents current restore job status.
// @Description RestoreJobStatus represents current restore job status.
type RestoreJobStatus struct {
	// Number of records read from backup.
	ReadRecords uint64 `yaml:"read-records,omitempty" json:"read-records,omitempty" format:"int64" example:"10"`
	// Total bytes read from backup.
	TotalBytes uint64 `yaml:"total-bytes,omitempty" json:"total-bytes,omitempty" format:"int64" example:"2000"`
	// The number of records dropped because they were expired.
	ExpiredRecords uint64 `yaml:"expired-records,omitempty" json:"expired-records,omitempty" format:"int64" example:"2"`
	// The number of records dropped because they didn't contain any of the
	// selected bins or didn't belong to any of the selected sets.
	SkippedRecords uint64 `yaml:"skipped-records,omitempty" json:"skipped-records,omitempty" format:"int64" example:"4"`
	// The number of records ignored because of a record-level permanent error while restoring.
	IgnoredRecords uint64 `yaml:"ignored-records,omitempty" json:"ignored-records,omitempty" format:"int64" example:"12"`
	// The number of successfully restored records.
	InsertedRecords uint64 `yaml:"inserted-records,omitempty" json:"inserted-records,omitempty" format:"int64" example:"8"`
	// The number of records dropped because they already existed in the database.
	ExistedRecords uint64 `yaml:"existed-records,omitempty" json:"existed-records,omitempty" format:"int64" example:"15"`
	// The number of records dropped because the database already contained the records with a higher generation count.
	FresherRecords uint64 `yaml:"fresher-records,omitempty" json:"fresher-records,omitempty" format:"int64" example:"5"`
	// The number of successfully created secondary indexes.
	IndexCount uint64 `yaml:"index-count,omitempty" json:"index-count,omitempty" format:"int64" example:"3"`
	// The number of successfully stored UDF files.
	UDFCount uint64 `yaml:"udf-count,omitempty" json:"udf-count,omitempty" format:"int64" example:"1"`

	// The number of errors in doubt while restoring.
	// (IsInDoubt signifies that the write operation may have gone through on the server
	// but the client is not able to confirm that due an error.)
	// Non zero value indicates that there are might be unexpected side effects during restore, like
	// * Generation counter greater than expected for some records.
	// * Fresher records counter greater than expected.
	ErrorsInDoubt uint64 `yaml:"errors-in-doubt,omitempty" json:"errors-in-doubt,omitempty" format:"int64" example:"7"`

	// Speed related metrics of the restore process.
	CurrentRestore *RunningJob `yaml:"current-restore,omitempty" json:"current-job,omitempty"`
	// Status of the restore job.
	Status JobStatus `yaml:"status,omitempty" json:"status,omitempty"`
	// Error message if any.
	Error string `yaml:"error,omitempty" json:"error,omitempty"`
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
	r.ReadRecords = m.Counters.GetReadRecords()
	r.TotalBytes = m.Counters.GetTotalBytesRead()
	r.ExpiredRecords = m.Counters.GetRecordsExpired()
	r.SkippedRecords = m.Counters.GetRecordsSkipped()
	r.IgnoredRecords = m.Counters.GetRecordsIgnored()
	r.InsertedRecords = m.Counters.GetRecordsInserted()
	r.ExistedRecords = m.Counters.GetRecordsExisted()
	r.FresherRecords = m.Counters.GetRecordsFresher()
	r.IndexCount = uint64(m.Counters.GetSIndexes())
	r.UDFCount = uint64(m.Counters.GetUDFs())
	r.Status = JobStatus(m.Status)
	r.ErrorsInDoubt = m.Counters.GetErrorsInDoubt()

	if m.Error != nil {
		r.Error = m.Error.Error()
	}
	r.CurrentRestore = NewRunningJobFromModel(m.CurrentRestore)
}
