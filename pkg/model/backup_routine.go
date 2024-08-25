package model

// BackupRoutine represents a scheduled backup operation routine.
// @Description BackupRoutine represents a scheduled backup operation routine.
//
//nolint:lll
type BackupRoutine struct {
	// The name of the corresponding backup policy.
	BackupPolicy *BackupPolicy
	// The name of the corresponding source cluster.
	SourceCluster *AerospikeCluster
	// The name of the corresponding storage provider configuration.
	Storage *Storage
	// The Secret Agent configuration for the routine (optional).
	SecretAgent *SecretAgent
	// The interval for full backup as a cron expression string.
	IntervalCron string
	// The interval for incremental backup as a cron expression string (optional).
	IncrIntervalCron string
	// The list of the namespaces to back up (optional, empty list implies backup up whole cluster).
	Namespaces []string
	// The list of backup set names (optional, an empty list implies backing up all sets).
	SetList []string
	// The list of backup bin names (optional, an empty list implies backing up all bins).
	BinList []string
	// A list of Aerospike Server rack IDs to prefer when reading records for a backup.
	PreferRacks []int
	// Back up list of partition filters. Partition filters can be ranges, individual partitions,
	// or records after a specific digest within a single partition.
	// Default number of partitions to back up: 0 to 4095: all partitions.
	PartitionList *string
}
