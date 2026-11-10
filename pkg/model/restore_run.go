package model

import "github.com/aerospike/backup-go/models"

// RestoreRun is a snapshot of a restore run, as the handler that runs it reports it.
type RestoreRun struct {
	// Counters are the counts of the run so far.
	Counters RestoreCounters
	// Metrics is the client-side throughput. It is nil when the backup mode measures none.
	Metrics *models.Metrics
	// Progress is the fraction of the run done, in the range [0, 1], for a run whose handler
	// learns it directly. It is nil when the progress follows from the record counts.
	Progress *float64
}

// RestoreCounters counts what a restore has read and written.
type RestoreCounters struct {
	// ReadRecords is the number of records read from the backup.
	ReadRecords uint64
	// InsertedRecords is the number of records written to the cluster.
	InsertedRecords uint64
	// SkippedRecords is the number of records left out by the namespace, set or bin filters.
	SkippedRecords uint64
	// ExpiredRecords is the number of records left out because they had expired.
	ExpiredRecords uint64
	// IgnoredRecords is the number of records whose write failed with an error the policy ignores.
	IgnoredRecords uint64
	// ExistedRecords is the number of records left out because the cluster already held them.
	ExistedRecords uint64
	// FresherRecords is the number of records left out because the cluster held a newer version.
	FresherRecords uint64
	// ErrorsInDoubt is the number of writes whose outcome is unknown.
	ErrorsInDoubt uint64
	// TotalBytesRead is the number of bytes read from the backup.
	TotalBytesRead uint64
	// BytesWritten is the number of bytes written to the cluster.
	BytesWritten uint64
	// SecondaryIndexes is the number of secondary indexes restored.
	SecondaryIndexes uint64
	// UDFs is the number of UDF files restored.
	UDFs uint64
}

// Plus returns the counters of c and other added up.
func (c RestoreCounters) Plus(other RestoreCounters) RestoreCounters {
	return RestoreCounters{
		ReadRecords:      c.ReadRecords + other.ReadRecords,
		InsertedRecords:  c.InsertedRecords + other.InsertedRecords,
		SkippedRecords:   c.SkippedRecords + other.SkippedRecords,
		ExpiredRecords:   c.ExpiredRecords + other.ExpiredRecords,
		IgnoredRecords:   c.IgnoredRecords + other.IgnoredRecords,
		ExistedRecords:   c.ExistedRecords + other.ExistedRecords,
		FresherRecords:   c.FresherRecords + other.FresherRecords,
		ErrorsInDoubt:    c.ErrorsInDoubt + other.ErrorsInDoubt,
		TotalBytesRead:   c.TotalBytesRead + other.TotalBytesRead,
		BytesWritten:     c.BytesWritten + other.BytesWritten,
		SecondaryIndexes: c.SecondaryIndexes + other.SecondaryIndexes,
		UDFs:             c.UDFs + other.UDFs,
	}
}
