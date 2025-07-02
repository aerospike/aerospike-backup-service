package dto

import (
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// RestorePolicy represents a policy for the restore operation.
// @Description RestorePolicy represents a policy for the restore operation.
type RestorePolicy struct {
	// The number of concurrent record readers from backup files.
	// This value controls the level of parallelism used by the backup service when
	// reading backup files.
	// The optimal value depends on hardware and network configuration.
	Parallel *int `json:"parallel,omitempty" example:"8" default:"8"`
	// Do not restore any record data (metadata or bin data).
	// By default, record data, secondary index definitions, and UDF modules will be restored.
	NoRecords *bool `json:"no-records,omitempty" default:"false"`
	// Do not restore any secondary index definitions.
	NoIndexes *bool `json:"no-indexes,omitempty" default:"false"`
	// Do not restore any UDF modules.
	NoUdfs *bool `json:"no-udfs,omitempty" default:"false"`
	// Timeout (ms) for Aerospike commands to write records, create indexes and create UDFs.
	// Socket timeout in milliseconds. Default is 10 minutes. If this value is 0, it is set to total-timeout.
	// If both are 0, there is no socket idle time limit.
	SocketTimeout *int64 `yaml:"socket-timeout,omitempty" json:"socket-timeout,omitempty" example:"1000" default:"60000"`
	// Total socket timeout in milliseconds. Default is 0, that is, no timeout.
	TotalTimeout *int64 `yaml:"total-timeout,omitempty" json:"total-timeout,omitempty" example:"2000" default:"0"`
	// Disables the use of batch writes when restoring records to the Aerospike cluster.
	// By default, the cluster is checked for batch write support.
	DisableBatchWrites *bool `json:"disable-batch-writes,omitempty" default:"false"`
	// The max number of outstanding async record batch write calls at a time.
	MaxAsyncBatches *int `json:"max-async-batches,omitempty" example:"32" default:"128"`
	// The max allowed number of records per an async batch write call.
	// Only applicable when using batch writes.
	BatchSize *int `json:"batch-size,omitempty" example:"32" default:"128"`
	// Namespace optionally specifies an alternative namespace name for the restore operation.
	// By default, the data is restored to the namespace from which it was taken.
	Namespace *RestoreNamespace `json:"namespace,omitempty"`
	// The sets to restore (optional, an empty list implies restoring all sets).
	SetList []string `json:"set-list,omitempty" example:"set1,set2" extensions:"x-nullable"`
	// The bins to restore (optional, an empty list implies restoring all bins).
	BinList []string `json:"bin-list,omitempty" example:"bin1,bin2"  extensions:"x-nullable"`
	// Replace records. This controls how records from the backup overwrite existing records in
	// the namespace. By default, restoring a record from a backup only replaces the bins
	// contained in the backup; all other bins of an existing record remain untouched.
	Replace *bool `json:"replace,omitempty" default:"false"`
	// Existing records take precedence. With this option, only records that do not exist in
	// the namespace are restored, regardless of generation numbers. If a record exists in
	// the namespace, the record from the backup is ignored.
	Unique *bool `json:"unique,omitempty" default:"false"`
	// Records from backups take precedence. This option disables the generation check.
	// With this option, records from the backup always overwrite records that already exist in
	// the namespace, regardless of generation numbers.
	NoGeneration *bool `json:"no-generation,omitempty" default:"false"`
	// Throttles read operations from the backup file(s) to not exceed the given I/O bandwidth in MiB/s.
	// Default: no limit.
	Bandwidth *int `json:"bandwidth,omitempty" example:"50000" extensions:"x-nullable" minimum:"8"`
	// Throttles read operations from the backup file(s) to not exceed the given number of transactions per second.
	// Default: no limit.
	Tps *int `json:"tps,omitempty" example:"4000" extensions:"x-nullable"`
	// Encryption details (algorithm and key). Default is no encryption.
	EncryptionPolicy *EncryptionPolicy `yaml:"encryption,omitempty" json:"encryption,omitempty"`
	// Compression details (algorithm). Default is no compression.
	CompressionPolicy *CompressionPolicy `yaml:"compression,omitempty" json:"compression,omitempty"`
	// Configuration of retries for each restore write operation.
	// If nil, the default policy is used (5 retries with a one-minute delay between attempts).
	RetryPolicy *RetryPolicy `yaml:"retry-policy,omitempty" json:"retry-policy,omitempty"`
	// Amount of extra time-to-live to add to records that have expirable void-times.
	// Must be set in seconds.
	ExtraTTL *int64 `yaml:"extra-ttl" json:"extra-ttl,omitempty" example:"86400" default:"0"`
}

// Validate validates the restore policy.
func (p *RestorePolicy) Validate() error {
	if p == nil {
		return nil
	}
	if err := validateBandwidth(p.Bandwidth); err != nil {
		return err
	}
	if p.Parallel != nil && *p.Parallel <= 0 {
		return errValidationNonPositive("parallel", *p.Parallel)
	}
	if p.TotalTimeout != nil && *p.TotalTimeout < 0 {
		return errValidationNegative("total-timeout", *p.TotalTimeout)
	}
	if p.SocketTimeout != nil && *p.SocketTimeout < 0 {
		return errValidationNegative("socket-timeout", *p.SocketTimeout)
	}
	if p.MaxAsyncBatches != nil && *p.MaxAsyncBatches <= 0 {
		return errValidationNonPositive("max-async-batches", *p.MaxAsyncBatches)
	}
	if p.BatchSize != nil && *p.BatchSize <= 0 {
		return errValidationNonPositive("batch-size", *p.BatchSize)
	}
	if p.Tps != nil && *p.Tps <= 0 {
		return errValidationNonPositive("tps", *p.Tps)
	}

	if err := p.validateExistingRecordPolicy(); err != nil {
		return err
	}

	if p.Namespace != nil { // namespace is optional.
		if err := p.Namespace.Validate(); err != nil {
			return fmt.Errorf("restore namespace invalid: %w", err)
		}
	}
	if err := p.EncryptionPolicy.Validate(); err != nil {
		return err
	}
	if err := p.CompressionPolicy.Validate(); err != nil {
		return err
	}
	if err := p.RetryPolicy.Validate(); err != nil {
		return fmt.Errorf("retry policy invalid: %w", err)
	}
	if p.ExtraTTL != nil && *p.ExtraTTL < 0 {
		return errValidationNegative("extra-ttl", *p.ExtraTTL)
	}

	return nil
}

func (p *RestorePolicy) validateExistingRecordPolicy() error {
	if p.Replace != nil && *p.Replace && p.Unique != nil && *p.Unique {
		return errValidationMutuallyExclusive("replace", "unique")
	}
	if p.NoGeneration != nil && *p.NoGeneration && p.Unique != nil && *p.Unique {
		return errValidationMutuallyExclusive("no-generation", "unique")
	}

	return nil
}

func (p *RestorePolicy) ToModel() *model.RestorePolicy {
	if p == nil {
		return &model.RestorePolicy{}
	}

	return &model.RestorePolicy{
		Parallel:           p.Parallel,
		NoRecords:          p.NoRecords,
		NoIndexes:          p.NoIndexes,
		NoUdfs:             p.NoUdfs,
		TotalTimeout:       millisToDuration(p.TotalTimeout),
		SocketTimeout:      millisToDuration(p.SocketTimeout),
		DisableBatchWrites: p.DisableBatchWrites,
		MaxAsyncBatches:    p.MaxAsyncBatches,
		BatchSize:          p.BatchSize,
		Namespace:          p.Namespace.ToModel(),
		SetList:            p.SetList,
		BinList:            p.BinList,
		Replace:            p.Replace,
		Unique:             p.Unique,
		NoGeneration:       p.NoGeneration,
		Bandwidth:          p.Bandwidth,
		Tps:                p.Tps,
		EncryptionPolicy:   p.EncryptionPolicy.ToModel(),
		CompressionPolicy:  p.CompressionPolicy.ToModel(),
		RetryPolicy:        p.RetryPolicy.ToModel(),
		ExtraTTL:           p.ExtraTTL,
	}
}
