package model

import (
	"fmt"
)

const (
	MIN_FULL_BACKUP_INTERVAL int64 = 10000
	MIN_INCR_BACKUP_INTERVAL int64 = 1000
)

// BackupRoutine represents a scheduled backup operation routine.
type BackupRoutine struct {
	BackupPolicy  string  `yaml:"backup-policy,omitempty" json:"backup-policy,omitempty"`
	SourceCluster string  `yaml:"source-cluster,omitempty" json:"source-cluster,omitempty"`
	Storage       string  `yaml:"storage,omitempty" json:"storage,omitempty"`
	SecretAgent   *string `yaml:"secret-agent,omitempty" json:"secret-agent,omitempty"`

	IntervalMillis     *int64 `yaml:"interval,omitempty" json:"interval,omitempty"`
	IncrIntervalMillis *int64 `yaml:"incr-interval,omitempty" json:"incr-interval,omitempty"`

	Namespace string   `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	SetList   []string `yaml:"set-list,omitempty" json:"set-list,omitempty"`
	BinList   []string `yaml:"bin-list,omitempty" json:"bin-list,omitempty"`
	NodeList  []Node   `yaml:"node-list,omitempty" json:"node-list,omitempty"`

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
		return routineValidationError("IntervalMillis")
	}
	if *r.IntervalMillis < MIN_FULL_BACKUP_INTERVAL {
		return fmt.Errorf("minimum backup interval is %d", MIN_FULL_BACKUP_INTERVAL)
	}
	if r.IncrIntervalMillis != nil && *r.IncrIntervalMillis < MIN_INCR_BACKUP_INTERVAL {
		return fmt.Errorf("minimum incremental backup interval is %d", MIN_INCR_BACKUP_INTERVAL)
	}
	return nil
}

func routineValidationError(field string) error {
	return fmt.Errorf("%s specification for backup routine is required", field)
}

// Node represents an Aerospike node details.
type Node struct {
	IP   string `yaml:"ip" json:"ip"`
	Port int    `yaml:"port" json:"port"`
}
