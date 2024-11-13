package model

import (
	"github.com/aerospike/aerospike-backup-service/v2/pkg/util"
	"github.com/aerospike/backup-go/models"
)

const (
	KeepAll           RemoveFilesType = "KeepAll"
	RemoveAll         RemoveFilesType = "RemoveAll"
	RemoveIncremental RemoveFilesType = "RemoveIncremental"
)

// RemoveFilesType represents the type of the backup storage.
// @Description RemoveFilesType represents the type of the backup storage.
type RemoveFilesType string

// BackupPolicy represents a scheduled backup policy.
type BackupPolicy struct {
	// Maximum number of scan calls to run in parallel.
	Parallel *int
	// Socket timeout in milliseconds. If this value is 0, it is set to total-timeout.
	// If both are 0, there is no socket idle time limit.
	SocketTimeout *int32
	// Total socket timeout in milliseconds. Default is 0, that is, no timeout.
	TotalTimeout *int32
	// RetryPolicy defines the configuration for retry attempts in case of failures.
	RetryPolicy *models.RetryPolicy
	// Whether to clear the output directory (default: KeepAll).
	RemoveFiles *RemoveFilesType
	// Do not back up any record data (metadata or bin data).
	NoRecords *bool
	// Do not back up any secondary index definitions.
	NoIndexes *bool
	// Do not back up any UDF modules.
	NoUdfs *bool
	// Throttles backup write operations to the backup file(s) to not exceed the given
	// bandwidth in MiB/s.
	Bandwidth *int64
	// Limit total returned records per second (RPS). If RPS is zero (the default),
	// the records-per-second limit is not applied.
	RecordsPerSecond *int32
	// File size limit (in MB) for the backup directory. If an .asb backup file crosses this size threshold,
	// a new backup file will be created.
	FileLimit *int64
	// Encryption details.
	EncryptionPolicy *EncryptionPolicy
	// Compression details.
	CompressionPolicy *CompressionPolicy
	// Sealed determines whether backup should include keys updated during the backup process.
	// When true, the backup contains only records that last modified before backup started.
	// When false (default), records updated during backup might be included in the backup, but it's not guaranteed.
	Sealed *bool
}

// IsSealed returns the value of the Sealed property.
// If the property is not set, it returns the default value.
func (p *BackupPolicy) IsSealed() bool {
	if p.Sealed != nil {
		return *p.Sealed
	}
	return defaultConfig.backupPolicy.sealed
}

// CopySMDDisabled creates a new instance of the BackupPolicy struct with identical field values.
// New instance has NoIndexes and NoUdfs set to true.
func (p *BackupPolicy) CopySMDDisabled() *BackupPolicy {
	return &BackupPolicy{
		Parallel:         p.Parallel,
		SocketTimeout:    p.SocketTimeout,
		TotalTimeout:     p.TotalTimeout,
		RetryPolicy:      p.RetryPolicy,
		RemoveFiles:      p.RemoveFiles,
		NoRecords:        p.NoRecords,
		NoIndexes:        util.Ptr(true),
		NoUdfs:           util.Ptr(true),
		Bandwidth:        p.Bandwidth,
		RecordsPerSecond: p.RecordsPerSecond,
		FileLimit:        p.FileLimit,
		Sealed:           p.Sealed,
	}
}

func (r *RemoveFilesType) RemoveFullBackup() bool {
	// Full backups are deleted only if RemoveFiles is explicitly set to RemoveAll
	return r != nil && *r == RemoveAll
}

func (r *RemoveFilesType) RemoveIncrementalBackup() bool {
	// Incremental backups are deleted only if RemoveFiles is explicitly set to RemoveAll or RemoveIncremental
	return r != nil && (*r == RemoveIncremental || *r == RemoveAll)
}
