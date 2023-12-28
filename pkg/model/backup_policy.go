package model

import "github.com/aerospike/backup/pkg/util"

// BackupPolicy represents a scheduled backup policy.
// @Description BackupPolicy represents a scheduled backup policy.
type BackupPolicy struct {
	// Maximum number of scan calls to run in parallel.
	Parallel *int32 `yaml:"parallel,omitempty" json:"parallel,omitempty"`
	// Socket timeout in milliseconds. If this value is 0, it is set to total-timeout.
	// If both are 0, there is no socket idle time limit.
	SocketTimeout *uint32 `yaml:"socket-timeout,omitempty" json:"socket-timeout,omitempty"`
	// Total socket timeout in milliseconds. Default is 0, that is, no timeout.
	TotalTimeout *uint32 `yaml:"total-timeout,omitempty" json:"total-timeout,omitempty"`
	// Maximum number of retries before aborting the current transaction.
	MaxRetries *uint32 `yaml:"max-retries,omitempty" json:"max-retries,omitempty"`
	RetryDelay *uint32 `yaml:"retry-delay,omitempty" json:"retry-delay,omitempty"`
	// Whether to clear the output directory.
	RemoveFiles *bool `yaml:"remove-files,omitempty" json:"remove-files,omitempty"`
	// Clear directory or remove output file.
	RemoveArtifacts *bool `yaml:"remove-artifacts,omitempty" json:"remove-artifacts,omitempty"`
	// Only backup record metadata (digest, TTL, generation count, key).
	NoBins *bool `yaml:"no-bins,omitempty" json:"no-bins,omitempty"`
	// Do not back up any record data (metadata or bin data).
	NoRecords *bool `yaml:"no-records,omitempty" json:"no-records,omitempty"`
	// Do not back up any secondary index definitions.
	NoIndexes *bool `yaml:"no-indexes,omitempty" json:"no-indexes,omitempty"`
	// Do not back up any UDF modules.
	NoUdfs *bool `yaml:"no-udfs,omitempty" json:"no-udfs,omitempty"`
	// Throttles backup write operations to the backup file(s) to not exceed the given
	// bandwidth in MiB/s.
	Bandwidth *uint64 `yaml:"bandwidth,omitempty" json:"bandwidth,omitempty"`
	// An approximate limit for the number of records to process. Available in server 4.9 and above.
	MaxRecords *uint64 `yaml:"max-records,omitempty" json:"max-records,omitempty"`
	// Limit total returned records per second (RPS). If RPS is zero (the default),
	// the records-per-second limit is not applied.
	RecordsPerSecond *uint32 `yaml:"records-per-second,omitempty" json:"records-per-second,omitempty"`
	// File size limit (in MB) for --directory. If an .asb backup file crosses this size threshold,
	// a new backup file will be created.
	FileLimit *uint64 `yaml:"file-limit,omitempty" json:"file-limit,omitempty"`
	FilterExp *string `yaml:"filter-exp,omitempty" json:"filter-exp,omitempty"`
}

// CopySMDDisabled creates a new instance of the BackupPolicy struct with identical field values.
// New instance has NoIndexes and NoUdfs set to true.
func (p *BackupPolicy) CopySMDDisabled() *BackupPolicy {
	return &BackupPolicy{
		Parallel:         p.Parallel,
		SocketTimeout:    p.SocketTimeout,
		TotalTimeout:     p.TotalTimeout,
		MaxRetries:       p.MaxRetries,
		RetryDelay:       p.RetryDelay,
		RemoveFiles:      p.RemoveFiles,
		RemoveArtifacts:  p.RemoveArtifacts,
		NoBins:           p.NoBins,
		NoRecords:        p.NoRecords,
		NoIndexes:        util.Ptr(true),
		NoUdfs:           util.Ptr(true),
		Bandwidth:        p.Bandwidth,
		MaxRecords:       p.MaxRecords,
		RecordsPerSecond: p.RecordsPerSecond,
		FileLimit:        p.FileLimit,
		FilterExp:        p.FilterExp,
	}
}
