package dto

import (
	"errors"
	"fmt"
	"io"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
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
// @Description BackupPolicy represents a scheduled backup policy.
//
//nolint:lll
type BackupPolicy struct {
	// Maximum number of scan calls to run in parallel.
	Parallel *int32 `yaml:"parallel,omitempty" json:"parallel,omitempty" example:"1"`
	// Socket timeout in milliseconds. If this value is 0, it is set to total-timeout.
	// If both are 0, there is no socket idle time limit.
	SocketTimeout *int32 `yaml:"socket-timeout,omitempty" json:"socket-timeout,omitempty" example:"1000"`
	// Total socket timeout in milliseconds. Default is 0, that is, no timeout.
	TotalTimeout *int32 `yaml:"total-timeout,omitempty" json:"total-timeout,omitempty" example:"2000"`
	// Maximum number of retries before aborting the current transaction.
	MaxRetries *int32 `yaml:"max-retries,omitempty" json:"max-retries,omitempty" example:"3"`
	// RetryDelay defines the delay in milliseconds before retrying a failed operation.
	RetryDelay *int32 `yaml:"retry-delay,omitempty" json:"retry-delay,omitempty" example:"500"`
	// Whether to clear the output directory (default: KeepAll).
	RemoveFiles *RemoveFilesType `yaml:"remove-files,omitempty" json:"remove-files,omitempty" enums:"KeepAll,RemoveAll,RemoveIncremental"`
	// Do not back up any record data (metadata or bin data).
	NoRecords *bool `yaml:"no-records,omitempty" json:"no-records,omitempty"`
	// Do not back up any secondary index definitions.
	NoIndexes *bool `yaml:"no-indexes,omitempty" json:"no-indexes,omitempty"`
	// Do not back up any UDF modules.
	NoUdfs *bool `yaml:"no-udfs,omitempty" json:"no-udfs,omitempty"`
	// Throttles backup write operations to the backup file(s) to not exceed the given
	// bandwidth in MiB/s.
	Bandwidth *int64 `yaml:"bandwidth,omitempty" json:"bandwidth,omitempty" example:"10000"`
	// Limit total returned records per second (RPS). If RPS is zero (the default),
	// the records-per-second limit is not applied.
	RecordsPerSecond *int32 `yaml:"records-per-second,omitempty" json:"records-per-second,omitempty" example:"1000"`
	// File size limit (in MB) for the backup directory. If an .asb backup file crosses this size threshold,
	// a new backup file will be created.
	FileLimit *int64 `yaml:"file-limit,omitempty" json:"file-limit,omitempty" example:"1024"`
	// Encryption details.
	EncryptionPolicy *EncryptionPolicy `yaml:"encryption,omitempty" json:"encryption,omitempty"`
	// Compression details.
	CompressionPolicy *CompressionPolicy `yaml:"compression,omitempty" json:"compression,omitempty"`
	// Sealed determines whether backup should include keys updated during the backup process.
	// When true, the backup contains only records that last modified before backup started.
	// When false (default), records updated during backup might be included in the backup, but it's not guaranteed.
	Sealed *bool `yaml:"sealed,omitempty" json:"sealed,omitempty"`
}

// NewBackupPolicyFromReader creates a new BackupPolicy object from a given reader
func NewBackupPolicyFromReader(r io.Reader, format SerializationFormat) (*BackupPolicy, error) {
	b := &BackupPolicy{}
	if err := Deserialize(b, r, format); err != nil {
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
		return errors.New("backup policy is not specified")
	}
	if p.Parallel != nil && *p.Parallel <= 0 {
		return fmt.Errorf("parallel %d invalid, should be positive number", *p.Parallel)
	}
	if p.SocketTimeout != nil && *p.SocketTimeout <= 0 {
		return fmt.Errorf("socketTimeout %d invalid, should be positive number", *p.SocketTimeout)
	}
	if p.TotalTimeout != nil && *p.TotalTimeout <= 0 {
		return fmt.Errorf("totalTimeout %d invalid, should be positive number", *p.TotalTimeout)
	}
	if p.MaxRetries != nil && *p.MaxRetries < 0 {
		return fmt.Errorf("maxRetries %d invalid, should be positive number", *p.MaxRetries)
	}
	if p.RetryDelay != nil && *p.RetryDelay < 0 {
		return fmt.Errorf("retryDelay %d invalid, should be positive number", *p.RetryDelay)
	}
	if p.Bandwidth != nil && *p.Bandwidth <= 0 {
		return fmt.Errorf("bandwidth %d invalid, should be positive number", *p.Bandwidth)
	}
	if p.RecordsPerSecond != nil && *p.RecordsPerSecond <= 0 {
		return fmt.Errorf("recordsPerSecond %d invalid, should be positive number", *p.RecordsPerSecond)
	}
	if p.FileLimit != nil && *p.FileLimit <= 0 {
		return fmt.Errorf("fileLimit %d invalid, should be positive number", *p.FileLimit)
	}
	if p.RemoveFiles != nil &&
		*p.RemoveFiles != KeepAll && *p.RemoveFiles != RemoveAll && *p.RemoveFiles != RemoveIncremental {
		return fmt.Errorf("invalid RemoveFiles: %s. Possible values: KeepAll, RemoveAll, RemoveIncremental", *p.RemoveFiles)
	}
	if err := p.EncryptionPolicy.Validate(); err != nil {
		return err
	}
	if err := p.CompressionPolicy.Validate(); err != nil {
		return err
	}
	return nil
}

func (p *BackupPolicy) ToModel() *model.BackupPolicy {
	return &model.BackupPolicy{
		Parallel:          p.Parallel,
		SocketTimeout:     p.SocketTimeout,
		TotalTimeout:      p.TotalTimeout,
		MaxRetries:        p.MaxRetries,
		RetryDelay:        p.RetryDelay,
		RemoveFiles:       (*model.RemoveFilesType)(p.RemoveFiles),
		NoRecords:         p.NoRecords,
		NoIndexes:         p.NoIndexes,
		NoUdfs:            p.NoUdfs,
		Bandwidth:         p.Bandwidth,
		RecordsPerSecond:  p.RecordsPerSecond,
		FileLimit:         p.FileLimit,
		EncryptionPolicy:  p.EncryptionPolicy.ToModel(),
		CompressionPolicy: p.CompressionPolicy.ToModel(),
		Sealed:            p.Sealed,
	}
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
	p.SocketTimeout = m.SocketTimeout
	p.TotalTimeout = m.TotalTimeout
	p.MaxRetries = m.MaxRetries
	p.RetryDelay = m.RetryDelay
	p.RemoveFiles = (*RemoveFilesType)(m.RemoveFiles)
	p.NoRecords = m.NoRecords
	p.NoIndexes = m.NoIndexes
	p.NoUdfs = m.NoUdfs
	p.Bandwidth = m.Bandwidth
	p.RecordsPerSecond = m.RecordsPerSecond
	p.FileLimit = m.FileLimit
	if m.EncryptionPolicy != nil {
		p.EncryptionPolicy = &EncryptionPolicy{}
		p.EncryptionPolicy.FromModel(m.EncryptionPolicy)
	}
	if p.CompressionPolicy != nil {
		p.CompressionPolicy = &CompressionPolicy{}
		p.CompressionPolicy.fromModel(m.CompressionPolicy)
	}
	p.Sealed = m.Sealed
}
