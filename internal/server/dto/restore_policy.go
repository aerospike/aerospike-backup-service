package dto

import (
	"errors"
	"fmt"
	"github.com/aerospike/aerospike-backup-service/pkg/model"
)

// RestorePolicy represents a policy for the restore operation.
// @Description RestorePolicy represents a policy for the restore operation.
type RestorePolicy struct {
	// The number of concurrent record readers from backup files.
	Parallel *int32 `json:"parallel,omitempty" example:"8"`
	// Do not restore any record data (metadata or bin data).
	// By default, record data, secondary index definitions, and UDF modules
	// will be restored.
	NoRecords *bool `json:"no-records,omitempty"`
	// Do not restore any secondary index definitions.
	NoIndexes *bool `json:"no-indexes,omitempty"`
	// Do not restore any UDF modules.
	NoUdfs *bool `json:"no-udfs,omitempty"`
	// Timeout (ms) for Aerospike commands to write records, create indexes and create UDFs.
	Timeout *int32 `json:"timeout,omitempty" example:"1000"`
	// Disables the use of batch writes when restoring records to the Aerospike cluster.
	// By default, the cluster is checked for batch write support.
	DisableBatchWrites *bool `json:"disable-batch-writes,omitempty"`
	// The max number of outstanding async record batch write calls at a time.
	MaxAsyncBatches *int32 `json:"max-async-batches,omitempty" example:"32"`
	// The max allowed number of records per an async batch write call.
	// Default is 128 with batch writes enabled, or 16 without batch writes.
	BatchSize *int32 `json:"batch-size,omitempty" example:"128"`
	// Namespace details for the restore operation.
	// By default, the data is restored to the namespace from which it was taken.
	Namespace *RestoreNamespace `json:"namespace,omitempty"`
	// The sets to restore (optional, an empty list implies restoring all sets).
	SetList []string `json:"set-list,omitempty" example:"set1,set2"`
	// The bins to restore (optional, an empty list implies restoring all bins).
	BinList []string `json:"bin-list,omitempty" example:"bin1,bin2"`
	// Replace records. This controls how records from the backup overwrite existing records in
	// the namespace. By default, restoring a record from a backup only replaces the bins
	// contained in the backup; all other bins of an existing record remain untouched.
	Replace *bool `json:"replace,omitempty"`
	// Existing records take precedence. With this option, only records that do not exist in
	// the namespace are restored, regardless of generation numbers. If a record exists in
	// the namespace, the record from the backup is ignored.
	Unique *bool `json:"unique,omitempty"`
	// Records from backups take precedence. This option disables the generation check.
	// With this option, records from the backup always overwrite records that already exist in
	// the namespace, regardless of generation numbers.
	NoGeneration *bool `json:"no-generation,omitempty"`
	// Throttles read operations from the backup file(s) to not exceed the given I/O bandwidth in bytes/sec.
	Bandwidth *int64 `json:"bandwidth,omitempty" example:"50000"`
	// Throttles read operations from the backup file(s) to not exceed the given number of transactions
	// per second.
	Tps *int32 `json:"tps,omitempty" example:"4000"`
	// Encryption details.
	EncryptionPolicy *EncryptionPolicy `yaml:"encryption,omitempty" json:"encryption,omitempty"`
	// Compression details.
	CompressionPolicy *CompressionPolicy `yaml:"compression,omitempty" json:"compression,omitempty"`
	// Configuration of retries for each restore write operation.
	// If nil, default retry policy will be used.
	RetryPolicy *RetryPolicy `yaml:"retry-policy,omitempty" json:"retry-policy,omitempty"`
}

// Validate validates the restore policy.
func (p *RestorePolicy) Validate() error {
	if p == nil {
		return fmt.Errorf("restore policy is not specified")
	}
	if p.Parallel != nil && *p.Parallel <= 0 {
		return fmt.Errorf("parallel %d invalid, should be positive number", *p.Parallel)
	}
	if p.Timeout != nil && *p.Timeout <= 0 {
		return fmt.Errorf("timeout %d invalid, should be positive number", *p.Timeout)
	}
	if p.MaxAsyncBatches != nil && *p.MaxAsyncBatches <= 0 {
		return fmt.Errorf("maxAsyncBatches %d invalid, should be positive number", *p.MaxAsyncBatches)
	}
	if p.BatchSize != nil && *p.BatchSize <= 0 {
		return fmt.Errorf("batchSize %d invalid, should be positive number", *p.BatchSize)
	}
	if p.Bandwidth != nil && *p.Bandwidth <= 0 {
		return fmt.Errorf("bandwidth %d invalid, should be positive number", *p.Bandwidth)
	}
	if p.Tps != nil && *p.Tps <= 0 {
		return fmt.Errorf("tps %d invalid, should be positive number", *p.Tps)
	}
	if p.Replace != nil && *p.Replace && p.Unique != nil && *p.Unique {
		return errors.New("replace and unique options are contradictory")
	}

	if p.Namespace != nil { // namespace is optional.
		if err := p.Namespace.Validate(); err != nil {
			return err
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
	return nil
}

func (p *RestorePolicy) GetRetryPolicyOrDefault() *RetryPolicy {
	if p.RetryPolicy != nil {
		return p.RetryPolicy
	}

	return defaultRetry
}

func (r *RestorePolicy) ToModel() *model.RestorePolicy {
	if r == nil {
		return nil
	}

	retryPolicy := r.RetryPolicy.ToModel()
	if retryPolicy == nil {
		retryPolicy = defaultRetry.ToModel()
	}

	return &model.RestorePolicy{
		Parallel:           r.Parallel,
		NoRecords:          r.NoRecords,
		NoIndexes:          r.NoIndexes,
		NoUdfs:             r.NoUdfs,
		Timeout:            r.Timeout,
		DisableBatchWrites: r.DisableBatchWrites,
		MaxAsyncBatches:    r.MaxAsyncBatches,
		BatchSize:          r.BatchSize,
		Namespace:          r.Namespace.ToModel(),
		SetList:            r.SetList,
		BinList:            r.BinList,
		Replace:            r.Replace,
		Unique:             r.Unique,
		NoGeneration:       r.NoGeneration,
		Bandwidth:          r.Bandwidth,
		Tps:                r.Tps,
		EncryptionPolicy:   r.EncryptionPolicy.ToModel(),
		CompressionPolicy:  r.CompressionPolicy.ToModel(),
		RetryPolicy:        *retryPolicy,
	}
}
