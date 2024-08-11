package dto

// BackupRoutineDTO represents a scheduled backup operation routine.
// @Description BackupRoutineDTO represents a scheduled backup operation routine.
type BackupRoutineDTO struct {
	// The name of the corresponding backup policy.
	BackupPolicy string `json:"backup-policy,omitempty" example:"daily" validate:"required"`
	// The name of the corresponding source cluster.
	SourceCluster string `json:"source-cluster,omitempty" example:"testCluster" validate:"required"`
	// The name of the corresponding storage provider configuration.
	Storage string `json:"storage,omitempty" example:"aws" validate:"required"`
	// The Secret Agent configuration for the routine (optional).
	SecretAgent *string `json:"secret-agent,omitempty" example:"sa"`
	// The interval for full backup as a cron expression string.
	IntervalCron string `json:"interval-cron" example:"0 0 * * * *" validate:"required"`
	// The interval for incremental backup as a cron expression string (optional).
	IncrIntervalCron string `json:"incr-interval-cron" example:"*/10 * * * * *"`
	// The list of the namespaces to back up (optional, empty list implies backup up whole cluster).
	Namespaces []string `json:"namespaces,omitempty" example:"source-ns1"`
	// The list of backup set names (optional, an empty list implies backing up all sets).
	SetList []string `json:"set-list,omitempty" example:"set1"`
	// The list of backup bin names (optional, an empty list implies backing up all bins).
	BinList []string `json:"bin-list,omitempty" example:"dataBin"`
	// A list of Aerospike Server rack IDs to prefer when reading records for a backup.
	PreferRacks []int `json:"prefer-racks,omitempty" example:"0"`
	// Back up list of partition filters. Partition filters can be ranges, individual partitions,
	// or records after a specific digest within a single partition.
	// Default number of partitions to back up: 0 to 4095: all partitions.
	PartitionList *string `json:"partition-list,omitempty" example:"0-1000"`
}
