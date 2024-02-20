package model

import "errors"

// RestorePolicy represents a policy for the restore operation.
// @Description RestorePolicy represents a policy for the restore operation.
type RestorePolicy struct {
	// The number of client threads to spawn for writing to the cluster.
	Parallel *uint32 `json:"parallel,omitempty" example:"8"`
	// Do not restore any record data (metadata or bin data).
	// By default, record data, secondary index definitions, and UDF modules
	// will be restored.
	NoRecords *bool `json:"no-records,omitempty"`
	// Do not restore any secondary index definitions.
	NoIndexes *bool `json:"no-indexes,omitempty"`
	// Do not restore any UDF modules.
	NoUdfs *bool `json:"no-udfs,omitempty"`
	// Timeout (ms) for Aerospike commands to write records, create indexes and create UDFs.
	Timeout *uint32 `json:"timeout,omitempty" example:"1000"`
	// Disables the use of batch writes when restoring records to the Aerospike cluster.
	// By default, the cluster is checked for batch write support.
	DisableBatchWrites *bool `json:"disable-batch-writes,omitempty"`
	// The max number of outstanding async record batch write calls at a time.
	MaxAsyncBatches *uint32 `json:"max-async-batches,omitempty" example:"32"`
	// The max allowed number of records per an async batch write call.
	// Default is 128 with batch writes enabled, or 16 without batch writes.
	BatchSize *uint32 `json:"batch-size,omitempty" example:"128"`
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
	// Throttles read operations from the backup file(s) to not exceed the given I/O bandwidth
	// in MiB/s and its database write operations to not exceed the given number of transactions
	// per second.
	Bandwidth *uint64 `json:"bandwidth,omitempty" example:"50000"`
	// Throttles read operations from the backup file(s) to not exceed the given I/O bandwidth
	// in MiB/s and its database write operations to not exceed the given number of transactions
	// per second.
	Tps *uint32 `json:"tps,omitempty" example:"4000"`
	// Encryption details.
	EncryptionPolicy *EncryptionPolicy `yaml:"encryption,omitempty" json:"encryption,omitempty"`
	// Compression details.
	CompressionPolicy *CompressionPolicy `yaml:"compression,omitempty" json:"compression,omitempty"`
}

// RestoreNamespace specifies an alternative namespace name for the restore
// operation, where Source is the original namespace name and Destination is
// the namespace name to which the backup data is to be restored.
//
// @Description RestoreNamespace specifies an alternative namespace name for the restore
// @Description operation.
type RestoreNamespace struct {
	// Original namespace name.
	Source *string `json:"source,omitempty" example:"source-ns" validate:"required"`
	// Destination namespace name.
	Destination *string `json:"destination,omitempty" example:"destination-ns" validate:"required"`
}

// Validate validates the restore policy.
func (p *RestorePolicy) Validate() error {
	if p == nil {
		return errors.New("restore policy is not specified")
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
	return nil
}

// Validate validates the restore namespace.
func (n *RestoreNamespace) Validate() error {
	if n.Source == nil {
		return errors.New("source namespace is not specified")
	}
	if n.Destination == nil {
		return errors.New("destination namespace is not specified")
	}
	return nil
}
