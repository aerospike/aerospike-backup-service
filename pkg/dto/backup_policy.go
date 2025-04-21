package dto

import (
	"fmt"
	"io"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// BackupPolicy represents a scheduled backup policy.
// @Description BackupPolicy represents a scheduled backup policy.
type BackupPolicy struct {
	// Maximum number of scan calls to run in parallel.
	Parallel *int `yaml:"parallel,omitempty" json:"parallel,omitempty" example:"1"`
	// Socket timeout in milliseconds. Default is 10 seconds. If this value is 0, it is set to total-timeout.
	// If both are 0, there is no socket idle time limit.
	SocketTimeout *int64 `yaml:"socket-timeout,omitempty" json:"socket-timeout,omitempty" example:"1000"`
	// Total socket timeout in milliseconds. Default is 0, that is, no timeout.
	TotalTimeout *int64 `yaml:"total-timeout,omitempty" json:"total-timeout,omitempty" example:"2000"`
	// RetryPolicy defines the configuration for retry attempts in case of failures.
	// If nil, default policy is used.
	RetryPolicy *RetryPolicy `yaml:"retry-policy,omitempty" json:"retry-policy,omitempty"`
	// Specifies how long to retain full and incremental backups.
	RetentionPolicy *RetentionPolicy `yaml:"retention,omitempty" json:"retention,omitempty"`
	// Do not back up any record data (metadata or bin data). Default: false.
	NoRecords *bool `yaml:"no-records,omitempty" json:"no-records,omitempty"`
	// Do not back up any secondary index definitions. Default: false.
	NoIndexes *bool `yaml:"no-indexes,omitempty" json:"no-indexes,omitempty"`
	// Do not back up any UDF modules. Default: false.
	NoUdfs *bool `yaml:"no-udfs,omitempty" json:"no-udfs,omitempty"`
	// Back up Aerospike cluster configuration. Default: false.
	WithClusterConfig *bool `yaml:"with-cluster-configuration,omitempty" json:"with-cluster-configuration,omitempty"`
	// Throttles backup write operations to the backup file(s) to not exceed the given
	// bandwidth in MiB/s.
	Bandwidth *int `yaml:"bandwidth,omitempty" json:"bandwidth,omitempty" example:"10000"`
	// Limit total returned records per second (RPS). If RPS is zero (the default),
	// the records-per-second limit is not applied.
	RecordsPerSecond *int `yaml:"records-per-second,omitempty" json:"records-per-second,omitempty" example:"1000"`
	// File size limit (in MB) for the backup directory. If an .asb backup file crosses this size threshold,
	// a new backup file will be created.
	FileLimit *int `yaml:"file-limit,omitempty" json:"file-limit,omitempty" example:"1024"`
	// Encryption details.
	EncryptionPolicy *EncryptionPolicy `yaml:"encryption,omitempty" json:"encryption,omitempty"`
	// Compression details.
	CompressionPolicy *CompressionPolicy `yaml:"compression,omitempty" json:"compression,omitempty"`
	// Sealed determines whether backup should include keys updated during the backup process.
	// When true, the backup contains only records that last modified before backup started.
	// When false (default), records updated during backup might be included in the backup, but it's not guaranteed.
	// This parameter does not affect XDR backups (which always includes all keys).
	Sealed *bool `yaml:"sealed,omitempty" json:"sealed,omitempty"`
	// XDR configuration for MRT backups.
	XDRConfig *XDRConfig `yaml:"xdr,omitempty" json:"xdr,omitempty"`
	// Allows incremental backups to run concurrently.
	// When false (default), incremental backups are skipped if another backup for same routine is in progress.
	AllowConcurrentIncremental *bool `yaml:"allow_concurrent_incremental,omitempty" json:"allow_concurrent_incremental,omitempty"` //nolint:lll
}

// NewBackupPolicyFromReader creates a new BackupPolicy object from a given reader.
func NewBackupPolicyFromReader(r io.Reader, format decoder.SerializationFormat) (*BackupPolicy, error) {
	b := &BackupPolicy{}
	if err := decoder.Deserialize(b, r, format); err != nil {
		return nil, err
	}

	if err := b.Validate(); err != nil {
		return nil, err
	}

	return b, nil
}

// Validate checks if the BackupPolicy is valid and has feasible parameters for the backup to commence.
func (p *BackupPolicy) Validate() error {
	if p == nil {
		return nil
	}
	if p.Parallel != nil && *p.Parallel <= 0 {
		return errValidationNonPositive("parallel", *p.Parallel)
	}
	if p.SocketTimeout != nil && *p.SocketTimeout < 0 {
		return errValidationNegative("socket-timeout", *p.SocketTimeout)
	}
	if p.TotalTimeout != nil && *p.TotalTimeout < 0 {
		return errValidationNegative("total-timeout", *p.TotalTimeout)
	}
	if err := p.RetryPolicy.Validate(); err != nil {
		return fmt.Errorf("retryPolicy validation failed: %w", err)
	}
	if p.Bandwidth != nil && *p.Bandwidth <= 0 {
		return errValidationNonPositive("bandwidth", *p.Bandwidth)
	}
	if p.RecordsPerSecond != nil && *p.RecordsPerSecond <= 0 {
		return errValidationNonPositive("records-per-second", *p.RecordsPerSecond)
	}
	if p.FileLimit != nil && *p.FileLimit < 0 {
		return errValidationNegative("file-limit", *p.FileLimit)
	}
	if err := p.RetentionPolicy.Validate(); err != nil {
		return fmt.Errorf("invalid retention policy: %w", err)
	}
	if err := p.EncryptionPolicy.Validate(); err != nil {
		return err
	}
	if err := p.CompressionPolicy.Validate(); err != nil {
		return err
	}
	if err := p.XDRConfig.Validate(); err != nil {
		return fmt.Errorf("invalid xdr config: %w", err)
	}

	return nil
}

func (p *BackupPolicy) ToModel() *model.BackupPolicy {
	if p == nil {
		return &model.BackupPolicy{}
	}

	return &model.BackupPolicy{
		Parallel:                   p.Parallel,
		SocketTimeout:              millisToDuration(p.SocketTimeout),
		TotalTimeout:               millisToDuration(p.TotalTimeout),
		RetryPolicy:                p.RetryPolicy.ToModel(),
		RetentionPolicy:            p.RetentionPolicy.toModel(),
		NoRecords:                  p.NoRecords,
		NoIndexes:                  p.NoIndexes,
		NoUdfs:                     p.NoUdfs,
		WithClusterConfig:          p.WithClusterConfig,
		Bandwidth:                  p.Bandwidth,
		RecordsPerSecond:           p.RecordsPerSecond,
		FileLimit:                  p.FileLimit,
		EncryptionPolicy:           p.EncryptionPolicy.ToModel(),
		CompressionPolicy:          p.CompressionPolicy.ToModel(),
		Sealed:                     p.Sealed,
		XDRConfig:                  p.XDRConfig.ToModel(),
		AllowConcurrentIncremental: p.AllowConcurrentIncremental,
	}
}

func millisToDuration(ms *int64) *time.Duration {
	if ms == nil {
		return nil
	}
	duration := time.Duration(*ms) * time.Millisecond
	return &duration
}

func durationToMillis(duration *time.Duration) *int64 {
	if duration == nil {
		return nil
	}
	ms := duration.Milliseconds()
	return &ms
}

func NewBackupPolicyFromModel(m *model.BackupPolicy) *BackupPolicy {
	if m == nil {
		return nil
	}

	b := &BackupPolicy{}
	b.fromModel(m)
	return b
}

func (p *BackupPolicy) fromModel(m *model.BackupPolicy) {
	p.Parallel = m.Parallel
	p.SocketTimeout = durationToMillis(m.SocketTimeout)
	p.TotalTimeout = durationToMillis(m.TotalTimeout)
	p.RetryPolicy = newRetryPolicyFromModel(m.RetryPolicy)
	p.RetentionPolicy = newRetentionPolicyFromModel(m.RetentionPolicy)
	p.NoRecords = m.NoRecords
	p.NoIndexes = m.NoIndexes
	p.NoUdfs = m.NoUdfs
	p.WithClusterConfig = m.WithClusterConfig
	p.Bandwidth = m.Bandwidth
	p.RecordsPerSecond = m.RecordsPerSecond
	p.FileLimit = m.FileLimit
	if m.EncryptionPolicy != nil {
		p.EncryptionPolicy = &EncryptionPolicy{}
		p.EncryptionPolicy.FromModel(m.EncryptionPolicy)
	}
	if m.CompressionPolicy != nil {
		p.CompressionPolicy = &CompressionPolicy{}
		p.CompressionPolicy.fromModel(m.CompressionPolicy)
	}
	p.Sealed = m.Sealed
	p.XDRConfig = newXDRConfigFromModel(m.XDRConfig)
	p.AllowConcurrentIncremental = m.AllowConcurrentIncremental
}

// RetentionPolicy specifies how many full and incremental backups to keep.
type RetentionPolicy struct {
	// Number of full backups to store:
	// - If nil, retain all full backups.
	// - If N is specified, retain the last N full backups.
	// - The minimum value is 1.
	FullBackups *int `json:"full,omitempty" yaml:"full,omitempty"`

	// Number of full backups to store incremental backups for:
	// - If nil, retain all incremental backups.
	// - If N is specified, retain incremental backups for the last N full backups.
	// - If set to 0, do not retain any incremental backups.
	// - Must not exceed the value of FullBackups.
	IncrBackups *int `json:"incremental,omitempty" yaml:"incremental,omitempty"`
}

func (rp *RetentionPolicy) Validate() error {
	if rp == nil {
		return nil
	}
	if rp.FullBackups != nil && *rp.FullBackups < 1 {
		return fmt.Errorf("full backups retention %d is invalid, must be at least 1", *rp.FullBackups)
	}

	if rp.IncrBackups != nil {
		if *rp.IncrBackups < 0 {
			return errValidationNegative("incremental", *rp.IncrBackups)
		}
		if rp.FullBackups != nil && *rp.IncrBackups > *rp.FullBackups {
			return fmt.Errorf("incremental backups retention %d cannot exceed full backups retention %d",
				*rp.IncrBackups, *rp.FullBackups)
		}
	}

	return nil
}
func (rp *RetentionPolicy) toModel() *model.RetentionPolicy {
	if rp == nil {
		return nil
	}

	return &model.RetentionPolicy{
		FullBackups: rp.FullBackups,
		IncrBackups: rp.IncrBackups,
	}
}

func newRetentionPolicyFromModel(m *model.RetentionPolicy) *RetentionPolicy {
	if m == nil {
		return nil
	}

	return &RetentionPolicy{
		FullBackups: m.FullBackups,
		IncrBackups: m.IncrBackups,
	}
}
