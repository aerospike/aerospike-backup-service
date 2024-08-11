package dto

// BackupPolicyDTO represents a scheduled backup policy.
// @Description BackupPolicyDTO represents a scheduled backup policy.
type BackupPolicyDTO struct {
	// Maximum number of scan calls to run in parallel.
	Parallel *int32 `json:"parallel,omitempty" example:"1"`
	// Socket timeout in milliseconds. If this value is 0, it is set to total-timeout.
	// If both are 0, there is no socket idle time limit.
	SocketTimeout *int32 `json:"socket-timeout,omitempty" example:"1000"`
	// Total socket timeout in milliseconds. Default is 0, that is, no timeout.
	TotalTimeout *int32 `json:"total-timeout,omitempty" example:"2000"`
	// Maximum number of retries before aborting the current transaction.
	MaxRetries *int32 `json:"max-retries,omitempty" example:"3"`
	// RetryDelay defines the delay in milliseconds before retrying a failed operation.
	RetryDelay *int32 `json:"retry-delay,omitempty" example:"500"`
	// Whether to clear the output directory (default: KeepAll).
	RemoveFiles *string `json:"remove-files,omitempty" enums:"KeepAll,RemoveAll,RemoveIncremental"`
	// Clear directory or remove output file.
	RemoveArtifacts *bool `json:"remove-artifacts,omitempty"`
	// Only backup record metadata (digest, TTL, generation count, key).
	NoBins *bool `json:"no-bins,omitempty"`
	// Do not back up any record data (metadata or bin data).
	NoRecords *bool `json:"no-records,omitempty"`
	// Do not back up any secondary index definitions.
	NoIndexes *bool `json:"no-indexes,omitempty"`
	// Do not back up any UDF modules.
	NoUdfs *bool `json:"no-udfs,omitempty"`
	// Throttles backup write operations to the backup file(s) to not exceed the given
	// bandwidth in MiB/s.
	Bandwidth *int64 `json:"bandwidth,omitempty" example:"10000"`
	// An approximate limit for the number of records to process. Available in server 4.9 and above.
	MaxRecords *int64 `json:"max-records,omitempty" example:"10000"`
	// Limit total returned records per second (RPS). If RPS is zero (the default),
	// the records-per-second limit is not applied.
	RecordsPerSecond *int32 `json:"records-per-second,omitempty" example:"1000"`
	// File size limit (in MB) for the backup directory. If an .asb backup file crosses this size threshold,
	// a new backup file will be created.
	FileLimit *int64 `json:"file-limit,omitempty" example:"1024"`
	// Encryption details.
	EncryptionPolicy *EncryptionPolicyDTO `json:"encryption,omitempty"`
	// Compression details.
	CompressionPolicy *CompressionPolicyDTO `json:"compression,omitempty"`
	// Sealed determines whether backup should include keys updated during the backup process.
	// When true, the backup contains only records that last modified before backup started.
	// When false (default), records updated during backup might be included in the backup, but it's not guaranteed.
	Sealed *bool `json:"sealed,omitempty"`
}
