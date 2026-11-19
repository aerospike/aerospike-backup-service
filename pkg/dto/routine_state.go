package dto

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// RoutineState represent the current state of backups (full and incremental)
// @Description RoutineState represent the current state of backups (full and incremental).
type RoutineState struct {
	// Full represents the state of a full backup. Nil if no full backup is running.
	Full *RunningJob `json:"full,omitempty"`
	// Incremental represents the state of an incremental backup. Nil if no incremental backup is running.
	Incremental *RunningJob `json:"incremental,omitempty"`
	// LastFull: the time of the last successful full backup.
	// A nil value indicates that there has never been a full backup.
	LastFull *time.Time `json:"last-full,omitempty"`
	// LastIncremental: the time of the last successful incremental backup.
	// A nil value indicates that there has never been an incremental backup.
	LastIncremental *time.Time `json:"last-incremental,omitempty"`
	// NextFull: the time of the next scheduled full backup.
	NextFull *time.Time `json:"next-full,omitempty"`
	// NextIncremental: the time of the next scheduled incremental backup.
	NextIncremental *time.Time `json:"next-incremental,omitempty"`
	// LastFullJobs: the server jobs of the last finished full backup, one per namespace whose job
	// reported its status, whether it succeeded or not. Reported only in the server backup mode.
	// The list is kept in memory, so it is empty after a restart until a full backup finishes.
	LastFullJobs []ServerBackupJob `json:"last-full-jobs,omitempty"`
	// LastIncrementalJobs: the same for the last finished incremental backup.
	LastIncrementalJobs []ServerBackupJob `json:"last-incremental-jobs,omitempty"`
}

func NewRoutineStateFromModel(m model.RoutineState) RoutineState {
	return RoutineState{
		Full:                NewRunningJobFromModel(m.Full),
		Incremental:         NewRunningJobFromModel(m.Incremental),
		LastFull:            m.LastRunTime.FullBackupTime(),
		LastIncremental:     m.LastRunTime.IncrementalBackupTime(),
		NextFull:            m.NextRunTime.FullBackupTime(),
		NextIncremental:     m.NextRunTime.IncrementalBackupTime(),
		LastFullJobs:        newServerBackupJobsFromModel(m.LastFullJobs),
		LastIncrementalJobs: newServerBackupJobsFromModel(m.LastIncrementalJobs),
	}
}

// RunningJob tracks progress of currently running job.
// @Description RunningJob tracks progress of currently running job.
type RunningJob struct {
	// The total number of records to be processed.
	// Zero for a server-side backup, whose progress the server reports only as a percentage.
	TotalRecords uint64 `json:"total-records,omitempty" example:"100" extensions:"x-backup-mode=scan"`
	// The number of records that have been successfully backed up so far.
	// For a server-side backup, the count can exceed the number of distinct records.
	DoneRecords uint64 `json:"done-records" example:"50"`
	// The time when the operation started.
	StartTime time.Time `json:"start-time" example:"2006-01-02T15:04:05Z07:00"`
	// The time when the operation finished.
	// A nil value indicates that the operation is still running.
	FinishTime *time.Time `json:"finish-time,omitempty" example:"2006-01-02T15:04:05Z07:00"`
	// The progress of the backup operation as a percentage.
	// This value is not guaranteed to be accurate.
	PercentageDone uint `json:"percentage-done" example:"50"`
	// The estimated time when the backup operation will be completed.
	// It is calculated based on the current percentage done and duration.
	// A nil value indicates that the estimation is not available yet.
	// This value is not guaranteed to be accurate.
	EstimatedEndTime *time.Time `json:"estimated-end-time,omitempty" example:"2006-01-02T15:04:05Z07:00"`
	// Metrics provides real-time information about data flow performance.
	// A server-side backup shows the rates the server last reported.
	Metrics *Metrics `json:"metrics,omitempty"`
	// Duration represents the elapsed time taken by the job in seconds.
	Duration uint `yaml:"duration" json:"duration"`
	// The server jobs of a running server-side backup, in the order they started: one per
	// namespace whose job has reported its status, each as the server last reported it.
	ServerJobs []ServerBackupJob `json:"server-jobs,omitempty" extensions:"x-backup-mode=server"`
}

func NewRunningJobFromModel(m *model.RunningJob) *RunningJob {
	if m == nil {
		return nil
	}

	r := &RunningJob{}
	r.fromModel(m)
	return r
}

func (r *RunningJob) fromModel(m *model.RunningJob) {
	r.TotalRecords = m.TotalRecords
	r.DoneRecords = m.DoneRecords
	r.StartTime = m.StartTime
	r.FinishTime = m.FinishTime
	r.PercentageDone = uint(m.Progress * 100)
	r.EstimatedEndTime = m.EstimatedEndTime
	r.Metrics = NewMetricsFromModel(m.Metrics)
	r.ServerJobs = newServerBackupJobsFromModel(m.ServerJobs)
	if r.FinishTime != nil {
		r.Duration = uint(r.FinishTime.Sub(r.StartTime).Seconds())
	} else {
		r.Duration = uint(time.Since(r.StartTime).Seconds())
	}
}
