package model

import (
	"fmt"
)

const (
	minimumFullBackupIntervalMillis int64 = 10000
	minimumIncrBackupIntervalMillis int64 = 1000
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

	// The interval for full backup in milliseconds.
	IntervalMillis *int64 `yaml:"interval,omitempty" json:"interval,omitempty"`
	// The interval for incremental backup in milliseconds (optional).
	IncrIntervalMillis *int64 `yaml:"incr-interval,omitempty" json:"incr-interval,omitempty"`

	// The name of the namespace to back up.
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	// The list of backup set names (optional, an empty list implies backing up all sets).
	SetList []string `yaml:"set-list,omitempty" json:"set-list,omitempty"`
	// The list of backup bin names (optional, an empty list implies backing up all bins).
	BinList []string `yaml:"bin-list,omitempty" json:"bin-list,omitempty"`
	// The list of nodes in the Aerospike cluster to run the backup for.
	NodeList []Node `yaml:"node-list,omitempty" json:"node-list,omitempty"`

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
	if r.Namespace == "" {
		return routineValidationError("namespace")
	}
	if r.IntervalMillis == nil {
		return routineValidationError("interval")
	}
	if *r.IntervalMillis < minimumFullBackupIntervalMillis {
		return fmt.Errorf("minimum full backup interval is %d", minimumFullBackupIntervalMillis)
	}
	if r.IncrIntervalMillis != nil && *r.IncrIntervalMillis < minimumIncrBackupIntervalMillis {
		return fmt.Errorf("minimum incremental backup interval is %d", minimumIncrBackupIntervalMillis)
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
