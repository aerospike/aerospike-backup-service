package model

import (
	"time"

	"github.com/aerospike/backup-go/models"
)

// BackupRun is a snapshot of a routine's backup run, as the handler that runs it reports it.
// The fields every run has are at the top; what a backup mode alone knows goes in a slot of its
// own, set only by that mode.
type BackupRun struct {
	// StartTime is when the run started.
	StartTime time.Time
	// Progress is the fraction of the run done, in the range [0, 1].
	Progress float64
	// Scan is set by a scan backup, which reads the records itself.
	Scan *ScanProgress
	// Server is set by a server backup, which the server runs as one job per namespace.
	Server *ServerProgress
}

// ServerProgress is what a server backup knows about its run: the server jobs it asked for and
// the running job's throughput.
type ServerProgress struct {
	// Jobs are the server jobs of the run in the order they started, each as last reported.
	Jobs []ServerBackupJob
	// Metrics is the running job's throughput as the server last reported it: records read and
	// bytes uploaded per second. Zero-valued until the job reports. The server has no pipeline gauge.
	Metrics *models.Metrics
}

// RecordsBackedUp is the number of records the server's jobs report as backed up so far, as the
// server counts them: the jobs of finished namespaces and the running one's latest figure.
func (p ServerProgress) RecordsBackedUp() uint64 {
	var sum uint64
	for _, job := range p.Jobs {
		sum += job.RecordsBackedUp
	}

	return sum
}

// ScanProgress is what a scan backup knows about its run: the records it counts and the
// client-side throughput.
type ScanProgress struct {
	// TotalRecords is the estimated number of records to read, zero until estimated.
	TotalRecords uint64
	// DoneRecords is the number of records read so far.
	DoneRecords uint64
	// Metrics is the client-side throughput summed across the namespaces; zero-valued while
	// none of them reports any.
	Metrics *models.Metrics
}
