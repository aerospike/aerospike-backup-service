package dto

import (
	"fmt"
	"io"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aws/smithy-go/ptr"
	"github.com/reugn/go-quartz/quartz"
)

// BackupRoutine represents a scheduled backup operation routine.
// @Description BackupRoutine represents a scheduled backup operation routine.
//
//nolint:lll
type BackupRoutine struct {
	// The name of the corresponding backup policy.
	BackupPolicy string `yaml:"backup-policy,omitempty" json:"backup-policy,omitempty" example:"daily" validate:"required"`
	// The name of the corresponding source cluster.
	SourceCluster string `yaml:"source-cluster,omitempty" json:"source-cluster,omitempty" example:"testCluster" validate:"required"`
	// The name of the corresponding storage provider configuration.
	Storage string `yaml:"storage,omitempty" json:"storage,omitempty" example:"aws" validate:"required"`
	// The Secret Agent configuration for the routine (optional).
	SecretAgent *string `yaml:"secret-agent,omitempty" json:"secret-agent,omitempty" example:"sa"`
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

// Validate validates the backup routine configuration.
func (r *BackupRoutine) Validate() error {
	if r.BackupPolicy == "" {
		return emptyFieldValidationError("backup policy")
	}
	if r.SourceCluster == "" {
		return emptyFieldValidationError("source-cluster")
	}
	if r.Storage == "" {
		return emptyFieldValidationError("storage")
	}
	if err := quartz.ValidateCronExpression(r.IntervalCron); err != nil {
		return fmt.Errorf("backup interval string '%s' invalid: %w", r.IntervalCron, err)
	}
	if r.IncrIntervalCron != "" { // incremental interval is optional
		if err := quartz.ValidateCronExpression(r.IncrIntervalCron); err != nil {
			return fmt.Errorf("incremental backup interval string '%s' invalid: %w", r.IntervalCron, err)
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
	}
	return nil
}

func (r *BackupRoutine) ToModel(config *model.Config) (*model.BackupRoutine, error) {
	policy, found := config.BackupPolicies[r.BackupPolicy]
	if !found {
		return nil, notFoundValidationError("backup policy", r.BackupPolicy)
	}

	cluster, found := config.AerospikeClusters[r.SourceCluster]
	if !found {
		return nil, notFoundValidationError("Aerospike cluster", r.SourceCluster)
	}

	if cluster.MaxParallelScans != nil {
		if len(r.SetList) > *cluster.MaxParallelScans {
			return nil, fmt.Errorf("max parallel scans must be at least the cardinality of set-list")
		}
	}

	storage, found := config.Storage[r.Storage]
	if !found {
		return nil, notFoundValidationError("storage", r.Storage)
	}

	var secretAgent *model.SecretAgent
	if r.SecretAgent != nil {
		secretAgent, found = config.SecretAgents[*r.SecretAgent]
		if !found {
			return nil, notFoundValidationError("secret agent", *r.SecretAgent)
		}
	}

	return &model.BackupRoutine{
		BackupPolicy:     policy,
		SourceCluster:    cluster,
		Storage:          storage,
		SecretAgent:      secretAgent,
		IntervalCron:     r.IntervalCron,
		IncrIntervalCron: r.IncrIntervalCron,
		Namespaces:       r.Namespaces,
		SetList:          r.SetList,
		BinList:          r.BinList,
		PreferRacks:      r.PreferRacks,
		PartitionList:    r.PartitionList,
	}, nil
}

// NewRoutineFromReader creates a new BackupRoutine object from a given reader
func NewRoutineFromReader(r io.Reader, format SerializationFormat) (*BackupRoutine, error) {
	b := &BackupRoutine{}
	if err := Deserialize(b, r, format); err != nil {
		return nil, err
	}

	if err := b.Validate(); err != nil {
		return nil, err
	}

	return b, nil
}

func NewRoutineFromModel(m *model.BackupRoutine, config *model.Config) *BackupRoutine {
	if m == nil || config == nil {
		return nil
	}

	b := &BackupRoutine{}
	b.fromModel(m, config)
	return b
}

func (r *BackupRoutine) fromModel(m *model.BackupRoutine, config *model.Config) {
	r.BackupPolicy = findKeyByValue(config.BackupPolicies, m.BackupPolicy)
	r.SourceCluster = findKeyByValue(config.AerospikeClusters, m.SourceCluster)
	r.Storage = findStorageKey(config.Storage, m.Storage)
	if m.SecretAgent != nil {
		r.SecretAgent = ptr.String(findKeyByValue(config.SecretAgents, m.SecretAgent))
	}
	r.IntervalCron = m.IntervalCron
	r.IncrIntervalCron = m.IncrIntervalCron
	r.Namespaces = m.Namespaces
	r.SetList = m.SetList
	r.BinList = m.BinList
	r.PreferRacks = m.PreferRacks
	r.PartitionList = m.PartitionList
}

func findKeyByValue[V any](m map[string]*V, value *V) string {
	for k, v := range m {
		if v == value {
			return k
		}
	}
	return ""
}

func findStorageKey(storageMap map[string]model.Storage, targetStorage model.Storage) string {
	for key, storage := range storageMap {
		if storage == targetStorage {
			return key
		}
	}
	return ""
}
