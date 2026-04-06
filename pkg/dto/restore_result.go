package dto

import (
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// RestoreJobStatus represents restore job status.
// @Description RestoreJobStatus represents restore job status.
type RestoreJobStatus struct {
	// Number of records read from backup.
	// When restore is finished, `read-records` =  `inserted-records` + `fresher-records` +
	// `existed-records` + `ignored-records` + `skipped-records` + `expired-records`
	ReadRecords uint64 `yaml:"read-records" json:"read-records" format:"int64" example:"10"`
	// Total bytes read from backup.
	TotalBytes uint64 `yaml:"total-bytes" json:"total-bytes" format:"int64" example:"2000"`
	// The number of records dropped because they were expired.
	ExpiredRecords uint64 `yaml:"expired-records" json:"expired-records" format:"int64" example:"2"`
	// The number of records dropped because they didn't contain any of the
	// selected bins or didn't belong to any of the selected sets.
	SkippedRecords uint64 `yaml:"skipped-records" json:"skipped-records" format:"int64" example:"4"`
	// The number of records ignored because of a record-level permanent error while restoring.
	IgnoredRecords uint64 `yaml:"ignored-records" json:"ignored-records" format:"int64" example:"12"`
	// The number of successfully restored records.
	InsertedRecords uint64 `yaml:"inserted-records" json:"inserted-records" format:"int64" example:"8"`
	// The number of records dropped because they already existed in the database.
	ExistedRecords uint64 `yaml:"existed-records" json:"existed-records" format:"int64" example:"15"`
	// The number of records dropped because the database already contained the records with a higher generation count.
	FresherRecords uint64 `yaml:"fresher-records" json:"fresher-records" format:"int64" example:"5"`
	// The number of successfully created secondary indexes.
	IndexCount uint64 `yaml:"index-count" json:"index-count" format:"int64" example:"3"`
	// The number of successfully stored UDF files.
	UDFCount uint64 `yaml:"udf-count" json:"udf-count" format:"int64" example:"1"`

	// The number of errors in doubt while restoring.
	// (IsInDoubt signifies that the write operation may have gone through on the server
	// but the client is not able to confirm that due an error.)
	// Non zero value indicates that there are might be unexpected side effects during restore, like
	// * Generation counter greater than expected for some records.
	// * Fresher records counter greater than expected.
	ErrorsInDoubt uint64 `yaml:"errors-in-doubt" json:"errors-in-doubt" format:"int64" example:"7"`

	// Speed related metrics of the restore process.
	CurrentRestore *RunningJob `yaml:"current-restore" json:"current-job"`
	// Status of the restore job.
	Status JobStatus `yaml:"status" json:"status"`
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
	r.Status = JobStatusFromModel(m.Status)
	r.ErrorsInDoubt = m.Counters.GetErrorsInDoubt()

	if m.Error != nil {
		r.Error = m.Error.Error()
	}
	r.CurrentRestore = NewRunningJobFromModel(m.CurrentRestore)
}
