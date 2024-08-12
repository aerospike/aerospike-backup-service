package model

import (
	"fmt"

	"github.com/reugn/go-quartz/quartz"
)

// BackupRoutine represents a scheduled backup operation routine.
// @Description BackupRoutine represents a scheduled backup operation routine.
//
//nolint:lll
type BackupRoutine struct {
	// The name of the corresponding backup policy.
	BackupPolicy string `yaml:"backup-policy,omitempty" example:"daily" validate:"required"`
	// The name of the corresponding source cluster.
	SourceCluster string `yaml:"source-cluster,omitempty" example:"testCluster" validate:"required"`
	// The name of the corresponding storage provider configuration.
	Storage string `yaml:"storage,omitempty" example:"aws" validate:"required"`
	// The Secret Agent configuration for the routine (optional).
	SecretAgent *string `yaml:"secret-agent,omitempty" example:"sa"`
	// The interval for full backup as a cron expression string.
	IntervalCron string `yaml:"interval-cron" example:"0 0 * * * *" validate:"required"`
	// The interval for incremental backup as a cron expression string (optional).
	IncrIntervalCron string `yaml:"incr-interval-cron" example:"*/10 * * * * *"`
	// The list of the namespaces to back up (optional, empty list implies backup up whole cluster).
	Namespaces []string `yaml:"namespaces,omitempty" example:"source-ns1"`
	// The list of backup set names (optional, an empty list implies backing up all sets).
	SetList []string `yaml:"set-list,omitempty" example:"set1"`
	// The list of backup bin names (optional, an empty list implies backing up all bins).
	BinList []string `yaml:"bin-list,omitempty" example:"dataBin"`
	// A list of Aerospike Server rack IDs to prefer when reading records for a backup.
	PreferRacks []int `yaml:"prefer-racks,omitempty" example:"0"`
	// Back up list of partition filters. Partition filters can be ranges, individual partitions,
	// or records after a specific digest within a single partition.
	// Default number of partitions to back up: 0 to 4095: all partitions.
	PartitionList *string `yaml:"partition-list,omitempty" example:"0-1000"`
}

// Validate validates the backup routine configuration.
func (r *BackupRoutine) Validate(c *Config) error {
	if r.BackupPolicy == "" {
		return emptyFieldValidationError("backup policy")
	}
	if _, exists := c.BackupPolicies[r.BackupPolicy]; !exists {
		return notFoundValidationError("backup policy", r.BackupPolicy)
	}
	if r.SourceCluster == "" {
		return emptyFieldValidationError("source-cluster")
	}
	if _, exists := c.AerospikeClusters[r.SourceCluster]; !exists {
		return notFoundValidationError("Aerospike cluster", r.SourceCluster)
	}
	if r.Storage == "" {
		return emptyFieldValidationError("storage")
	}
	if _, exists := c.Storage[r.Storage]; !exists {
		return notFoundValidationError("storage", r.Storage)
	}

	if err := quartz.ValidateCronExpression(r.IntervalCron); err != nil {
		return fmt.Errorf("backup interval string '%s' invalid: %v", r.IntervalCron, err)
	}
	if r.IncrIntervalCron != "" { // incremental interval is optional
		if err := quartz.ValidateCronExpression(r.IncrIntervalCron); err != nil {
			return fmt.Errorf("incremental backup interval string '%s' invalid: %v", r.IntervalCron, err)
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
	if r.SecretAgent != nil {
		if *r.SecretAgent == "" {
			return emptyFieldValidationError("secret-agent")
		}

		if _, exists := c.SecretAgents[*r.SecretAgent]; !exists {
			return notFoundValidationError("secret agent", *r.SecretAgent)
		}
	}
	return nil
}

// Node represents the Aerospike node details.
// @Description Node represents the Aerospike node details.
type Node struct {
	IP   string `yaml:"ip" json:"ip" example:"192.168.0.2"`
	Port int    `yaml:"port" json:"port" example:"3000"`
}
