package model

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/optional"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/aerospike/backup-go/models"
)

// BackupPolicy represents a scheduled backup policy.
type BackupPolicy struct {
	// Maximum number of scan calls to run in parallel.
	Parallel *int
	// Maximum number of threads to use for writing backup files.
	ParallelWrite *int
	// Socket timeout. If this value is 0, it is set to total-timeout.
	// If both are 0, there is no socket idle time limit.
	SocketTimeout *time.Duration
	// Total socket timeout. Default is 0, that is, no timeout.
	TotalTimeout *time.Duration
	// RetryPolicy defines the configuration for retry attempts in case of failures.
	RetryPolicy *RetryPolicy
	// Specifies how long to retain full and incremental backups.
	RetentionPolicy *RetentionPolicy
	// Do not back up any record data (metadata or bin data).
	NoRecords *bool
	// Do not back up any secondary index definitions.
	NoIndexes *bool
	// Do not back up any UDF modules.
	NoUdfs *bool
	// Back up Aerospike cluster configuration.
	WithClusterConfig *bool
	// Throttles backup write operations to the backup file(s) to not exceed the given
	// bandwidth in MiB/s. Zero (default) means no limit is applied.
	Bandwidth *int64
	// Limit total returned records per second (RPS). If RPS is zero (the default),
	// the records-per-second limit is not applied.
	RecordsPerSecond *int
	// File size limit (in MiB) for the backup directory. If an .asb backup file crosses this size threshold,
	// a new backup file will be created.
	FileLimit *int
	// Encryption details.
	EncryptionPolicy *EncryptionPolicy
	// Compression details.
	CompressionPolicy *CompressionPolicy
	// Sealed determines whether backup should include keys updated during the backup process.
	// When true, the backup contains only records that last modified before backup started.
	// When false (default), records updated during backup might be included in the backup, but it's not guaranteed.
	Sealed *bool
	// XDR configuration for MRT backups.
	// Commented out in dto.BackupPolicy, will always be nil.
	XDRConfig *XDRConfig
	// Allows incremental backups to run concurrently.
	ConcurrentIncremental *bool
	// Enables built-in compression during scan operation.
	UseCompression *bool
	// Maximum number of concurrent requests to server nodes.
	MaxConcurrentNodes *int
}

// IsSealedOrDefault returns the value of the Sealed property.
// If the property is not set, it returns the default value.
func (p *BackupPolicy) IsSealedOrDefault() bool {
	if p != nil && p.Sealed != nil {
		return *p.Sealed
	}

	return *defaultConfig.backupPolicy.Sealed
}

func (p *BackupPolicy) AllowConcurrentIncremental() bool {
	if p != nil && p.ConcurrentIncremental != nil {
		return *p.ConcurrentIncremental
	}

	return *defaultConfig.backupPolicy.ConcurrentIncremental
}

func (p *BackupPolicy) UseCompressionOrDefault() bool {
	if p != nil && p.UseCompression != nil {
		return *p.UseCompression
	}

	return *defaultConfig.backupPolicy.UseCompression
}

// CopyWithNoRecords creates a new instance of the BackupPolicy struct with identical field values.
// New instance has NoRecords set to true.
func (p *BackupPolicy) CopyWithNoRecords() *BackupPolicy {
	return &BackupPolicy{
		Parallel:         p.Parallel,
		ParallelWrite:    p.ParallelWrite,
		SocketTimeout:    p.SocketTimeout,
		TotalTimeout:     p.TotalTimeout,
		RetryPolicy:      p.RetryPolicy,
		RetentionPolicy:  p.RetentionPolicy,
		NoRecords:        ptr.Of(true),
		NoIndexes:        p.NoIndexes,
		NoUdfs:           p.NoUdfs,
		Bandwidth:        p.Bandwidth,
		RecordsPerSecond: p.RecordsPerSecond,
		FileLimit:        p.FileLimit,
		Sealed:           p.Sealed,
	}
}

func (p *BackupPolicy) GetRetryPolicyOrDefault() *models.RetryPolicy {
	if p != nil && p.RetryPolicy != nil {
		return p.RetryPolicy.Backup()
	}

	return defaultConfig.backupPolicy.RetryPolicy.Backup()
}

func (p *BackupPolicy) GetParallelOrDefault() int {
	if p.Parallel != nil {
		return *p.Parallel
	}

	return *defaultConfig.backupPolicy.Parallel
}

func (p *BackupPolicy) GetParallelWriteOrDefault() int {
	if p.ParallelWrite != nil {
		return *p.ParallelWrite
	}
	return p.GetParallelOrDefault()
}

func (p *BackupPolicy) GetFileLimitOrDefault() int {
	if p.FileLimit != nil {
		return *p.FileLimit
	}

	return *defaultConfig.backupPolicy.FileLimit
}

// RetentionPolicy defines the rules for automatically removing old full and incremental backups.
// This policy is applied by the Retention Manager after each successful full backup.
type RetentionPolicy struct {
	// FullBackups specifies the maximum number of the latest full backups to retain.
	// Not set: All full backups are retained.
	// Set to N >= 1: Only the last N successful full backups will be kept.
	FullBackups optional.Optional[int]

	// IncrBackups specifies the number of the latest full backups for which to retain
	// their associated incremental backup chains.
	// This field uses Optional[int] to distinguish three states:
	// 1. Not set: All incremental backups are retained.
	// 2. Set to 0: Explicitly keep NO incremental backups.
	// 3. Set to M > 0: Keep the incremental chain only for the last M full backups.
	//
	// Note: M must not exceed the value set for FullBackups (N).
	IncrBackups optional.Optional[int]
}

func (r *RetentionPolicy) IsEmpty() bool {
	if r == nil {
		return true
	}

	return !r.FullBackups.Present && !r.IncrBackups.Present
}
