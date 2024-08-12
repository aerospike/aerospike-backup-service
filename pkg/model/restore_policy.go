package model

// RestorePolicy represents a policy for the restore operation.
// @Description RestorePolicy represents a policy for the restore operation.
type RestorePolicy struct {
	// The number of client threads to spawn for writing to the cluster.
	Parallel *int32
	// Do not restore any record data (metadata or bin data).
	// By default, record data, secondary index definitions, and UDF modules
	// will be restored.
	NoRecords *bool
	// Do not restore any secondary index definitions.
	NoIndexes *bool
	// Do not restore any UDF modules.
	NoUdfs *bool
	// Timeout (ms) for Aerospike commands to write records, create indexes and create UDFs.
	Timeout *int32
	// Disables the use of batch writes when restoring records to the Aerospike cluster.
	// By default, the cluster is checked for batch write support.
	DisableBatchWrites *bool
	// The max number of outstanding async record batch write calls at a time.
	MaxAsyncBatches *int32
	// The max allowed number of records per an async batch write call.
	// Default is 128 with batch writes enabled, or 16 without batch writes.
	BatchSize *int32
	// Namespace details for the restore operation.
	// By default, the data is restored to the namespace from which it was taken.
	Namespace *RestoreNamespace
	// The sets to restore (optional, an empty list implies restoring all sets).
	SetList []string
	// The bins to restore (optional, an empty list implies restoring all bins).
	BinList []string
	// Replace records. This controls how records from the backup overwrite existing records in
	// the namespace. By default, restoring a record from a backup only replaces the bins
	// contained in the backup; all other bins of an existing record remain untouched.
	Replace *bool
	// Existing records take precedence. With this option, only records that do not exist in
	// the namespace are restored, regardless of generation numbers. If a record exists in
	// the namespace, the record from the backup is ignored.
	Unique *bool
	// Records from backups take precedence. This option disables the generation check.
	// With this option, records from the backup always overwrite records that already exist in
	// the namespace, regardless of generation numbers.
	NoGeneration *bool
	// Throttles read operations from the backup file(s) to not exceed the given I/O bandwidth in bytes/sec.
	Bandwidth *int64
	// Throttles read operations from the backup file(s) to not exceed the given number of transactions
	// per second.
	Tps *int32
	// Encryption details.
	EncryptionPolicy *EncryptionPolicy `yaml:"encryption,omitempty"`
	// Compression details.
	CompressionPolicy *CompressionPolicy `yaml:"compression,omitempty"`
}
