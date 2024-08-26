package model

// BackupRoutine represents a scheduled backup operation routine.
// @Description BackupRoutine represents a scheduled backup operation routine.
//
//nolint:lll
type BackupRoutine struct {
	// The name of the corresponding backup policy.
	BackupPolicy *BackupPolicy
	// The name of the corresponding source cluster.
	SourceCluster string `yaml:"source-cluster,omitempty" json:"source-cluster,omitempty" example:"testCluster" validate:"required"`
	// The name of the corresponding storage provider configuration.
	Storage *Storage
	// The Secret Agent configuration for the routine (optional).
	SecretAgent *SecretAgent
	// The interval for full backup as a cron expression string.
	IntervalCron string `yaml:"interval-cron" json:"interval-cron" example:"0 0 * * * *" validate:"required"`
	// The interval for incremental backup as a cron expression string (optional).
	IncrIntervalCron string `yaml:"incr-interval-cron" json:"incr-interval-cron" example:"*/10 * * * * *"`
	// The list of the namespaces to back up (optional, empty list implies backup up whole cluster).
	Namespaces []string `yaml:"namespaces,omitempty" json:"namespaces,omitempty" example:"source-ns1"`
	// The list of backup set names (optional, an empty list implies backing up all sets).
	SetList []string `yaml:"set-list,omitempty" json:"set-list,omitempty" example:"set1"`
	// The list of backup bin names (optional, an empty list implies backing up all bins).
	BinList []string `yaml:"bin-list,omitempty" json:"bin-list,omitempty" example:"dataBin"`
	// A list of Aerospike Server rack IDs to prefer when reading records for a backup.
	PreferRacks []int `yaml:"prefer-racks,omitempty" json:"prefer-racks,omitempty" example:"0"`
	// Back up list of partition filters. Partition filters can be ranges, individual partitions,
	// or records after a specific digest within a single partition.
	// Default number of partitions to back up: 0 to 4095: all partitions.
	PartitionList *string `yaml:"partition-list,omitempty" json:"partition-list,omitempty" example:"0-1000"`
}
