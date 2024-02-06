package model

import (
	"fmt"

	"github.com/reugn/go-quartz/quartz"
)

// BackupRoutine represents a scheduled backup operation routine.
// @Description BackupRoutine represents a scheduled backup operation routine.
type BackupRoutine struct {
	// The name of the corresponding bakup policy.
	BackupPolicy string `yaml:"backup-policy,omitempty" json:"backup-policy,omitempty"`
	// The name of the corresponding source cluster.
	SourceCluster string `yaml:"source-cluster,omitempty" json:"source-cluster,omitempty"`
	// The name of the corresponding storage provider configuration.
	Storage string `yaml:"storage,omitempty" json:"storage,omitempty"`
	// The Secret Agent configuration for the routine (optional).
	SecretAgent *string `yaml:"secret-agent,omitempty" json:"secret-agent,omitempty"`
	// The interval for full backup as a cron expression string.
	IntervalCron string `yaml:"interval-cron" json:"interval-cron"`
	// The interval for incremental backup as a cron expression string (optional).
	IncrIntervalCron string `yaml:"incr-interval-cron" json:"incr-interval-cron"`
	// The list of the namespaces to back up (optional, empty list implies backup up whole cluster).
	Namespaces []string `yaml:"namespaces,omitempty" json:"namespaces,omitempty"`
	// The list of backup set names (optional, an empty list implies backing up all sets).
	SetList []string `yaml:"set-list,omitempty" json:"set-list,omitempty"`
	// The list of backup bin names (optional, an empty list implies backing up all bins).
	BinList []string `yaml:"bin-list,omitempty" json:"bin-list,omitempty"`
	// The list of nodes in the Aerospike cluster to run the backup for.
	NodeList []Node `yaml:"node-list,omitempty" json:"node-list,omitempty"`
	// A list of Aerospike Server rack IDs to prefer when reading records for a backup.
	PreferRacks []int32 `yaml:"prefer-racks,omitempty" json:"prefer-racks,omitempty"`

	// Back up list of partition filters. Partition filters can be ranges, individual partitions,
	// or records after a specific digest within a single partition.
	// Default number of partitions to back up: 0 to 4095: all partitions.
	PartitionList *string `yaml:"partition-list,omitempty" json:"partition-list,omitempty"`
	AfterDigest   *string `yaml:"after-digest,omitempty" json:"after-digest,omitempty"`
}

// Validate validates the backup routine configuration.
func (r *BackupRoutine) Validate() error {
	if r.BackupPolicy == "" {
		return routineValidationError("backup-policy")
	}
	if r.SourceCluster == "" {
		return routineValidationError("source-cluster")
	}
	if r.Storage == "" {
		return routineValidationError("storage")
	}
	if err := quartz.ValidateCronExpression(r.IntervalCron); err != nil {
		return fmt.Errorf("backup interval string %s invalid: %v", r.IntervalCron, err)
	}
	if r.IncrIntervalCron != "" { // incremental interval is optional
		if err := quartz.ValidateCronExpression(r.IncrIntervalCron); err != nil {
			return fmt.Errorf("incremental backup interval string %s invalid: %v", r.IntervalCron, err)
		}
	}
	for _, rack := range r.PreferRacks {
		if rack < 0 {
			return fmt.Errorf("rack id %d invalid, should be positive number", rack)
		}
		if rack > maxRack {
			return fmt.Errorf("rack id %d invalid, should not exceed %d", rack, maxRack)
		}
	}
	return nil
}

func routineValidationError(field string) error {
	return fmt.Errorf("%s specification for backup routine is required", field)
}

// Node represents the Aerospike node details.
// @Description Node represents the Aerospike node details.
type Node struct {
	IP   string `yaml:"ip" json:"ip"`
	Port int    `yaml:"port" json:"port"`
}
