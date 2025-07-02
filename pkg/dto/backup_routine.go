package dto

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aws/smithy-go/ptr"
	"github.com/reugn/go-quartz/quartz"
)

// BackupRoutine represents a scheduled backup operation routine.
// @Description BackupRoutine represents a scheduled backup operation routine.
//
//nolint:lll
type BackupRoutine struct {
	// The name of the corresponding backup policy, one of defined in `config.backup-policies` (optional).
	BackupPolicy string `yaml:"backup-policy,omitempty" json:"backup-policy,omitempty" extensions:"x-nullable"`
	// The name of the corresponding source cluster.
	SourceCluster string `yaml:"source-cluster,omitempty" json:"source-cluster,omitempty" example:"testCluster" validate:"required"`
	// The name of the corresponding storage provider configuration.
	Storage string `yaml:"storage,omitempty" json:"storage,omitempty" validate:"required"`
	// The name of a Secret Agent to read secrets from (optional).
	SecretAgent *string `yaml:"secret-agent,omitempty" json:"secret-agent,omitempty" extensions:"x-nullable"`
	// The interval for full backup as a cron expression string.
	// Cron expression format: https://github.com/reugn/go-quartz?tab=readme-ov-file#cron-expression-format
	IntervalCron string `yaml:"interval-cron" json:"interval-cron" example:"0 0 * * * *" validate:"required"`
	// The interval for incremental backup as a cron expression string (optional).
	IncrIntervalCron string `yaml:"incr-interval-cron,omitempty" json:"incr-interval-cron,omitempty" example:"*/10 * * * * *" extensions:"x-nullable"`
	// The list of the namespaces to back up (empty list implies backup of the whole cluster).
	Namespaces *[]string `yaml:"namespaces,omitempty" json:"namespaces,omitempty" example:"[\"source-ns1\"]" validate:"required"`
	// The list of backup set names (optional, an empty list implies backing up all sets).
	SetList []string `yaml:"set-list,omitempty" json:"set-list,omitempty" example:"set1" extensions:"x-nullable"`
	// The list of backup bin names (optional, an empty list implies backing up all bins) extensions:"x-nullable".
	BinList []string `yaml:"bin-list,omitempty" json:"bin-list,omitempty" example:"dataBin" extensions:"x-nullable"`
	// The list of Aerospike Server rack IDs to prioritize when reading records during backup.
	// This is optional and can be used to optimize for rack-aware deployments.
	PreferRacks []int `yaml:"prefer-racks,omitempty" json:"prefer-racks,omitempty" example:"0" extensions:"x-nullable"`

	// PartitionList defines the list of partitions to include in the backup.
	// The format supports individual partitions or ranges.
	// - A range is specified as "<start>,<count>" (e.g., "100,50" backs up 50 partitions starting from 100).
	// - A single partition is specified as a number (e.g., "0").
	// Multiple entries can be comma-separated: e.g., "0,100,200,300,400,500".
	// By default, all partitions (0 to 4095) are backed up.
	// This field is mutually exclusive with node-list.
	PartitionList string `yaml:"partition-list,omitempty" json:"partition-list,omitempty" default:"0-4096"`

	// NodeList specifies which Aerospike nodes to include in the backup.
	// Only the listed nodes will be backed up.
	// Each node can be specified as one of the following:
	// - "<IP address>:<port>"
	// - "<hostname>:<port>"
	// - "<node ID>"
	// To obtain node identifiers, run: `asinfo -v "service:"`.
	// If using IP addresses or hostnames, ensure they match the values returned by the `asinfo` command.
	// This field is mutually exclusive with partition-list.
	// Parallelism is determined by the number of listed nodes unless `BackupPolicy.Parallel` is set to a lower value.
	NodeList []string `yaml:"node-list,omitempty" json:"node-list,omitempty" extensions:"x-nullable"`

	// Whether this routine is disabled and should not run. Default: false.
	Disabled bool `json:"disabled,omitempty" yaml:"disabled,omitempty" default:"false"`
}

// Validate validates the backup routine configuration.
func (r *BackupRoutine) Validate() error {
	if r.SourceCluster == "" {
		return errValidationEmptyField("source-cluster")
	}
	if r.Storage == "" {
		return errValidationEmptyField("storage")
	}
	if err := quartz.ValidateCronExpression(r.IntervalCron); err != nil {
		return fmt.Errorf("backup interval string '%s' invalid: %w", r.IntervalCron, err)
	}
	if r.IncrIntervalCron != "" { // incremental interval is optional
		if err := quartz.ValidateCronExpression(r.IncrIntervalCron); err != nil {
			return fmt.Errorf("incremental backup interval string '%s' invalid: %w", r.IntervalCron, err)
		}
	}
	for i, rack := range r.PreferRacks {
		if rack < 0 {
			return errValidationNegative(fmt.Sprintf("prefer-racks[%d]", i), rack)
		}
		if rack > maxRack {
			return fmt.Errorf("rack id %d invalid, should not exceed %d", rack, maxRack)
		}
	}
	if r.SecretAgent != nil {
		if *r.SecretAgent == "" {
			return errValidationEmptyField("secret-agent")
		}
	}
	if err := validatePartitionList(r.PartitionList); err != nil {
		return fmt.Errorf("invalid partition list: %q", r.PartitionList)
	}
	if len(r.PartitionList) > 0 && len(r.NodeList) > 0 {
		return errValidationMutuallyExclusive("partition-list", "node-list")
	}
	if r.Namespaces == nil {
		return errValidationEmptyField("namespaces")
	}

	return nil
}

func validatePartitionList(partitionList string) error {
	if partitionList == "" {
		return nil // empty list is valid
	}

	entries := strings.Split(partitionList, ",")
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			return fmt.Errorf("empty entry in partition list")
		}

		if err := validatePartitionEntry(entry); err != nil {
			return err
		}
	}

	return nil
}

func validatePartitionEntry(entry string) error {
	if strings.Contains(entry, "-") {
		return validatePartitionRange(entry)
	}

	if isValidPartitionID(entry) {
		return nil
	}

	return fmt.Errorf("invalid partition entry: %q", entry)
}

func validatePartitionRange(entry string) error {
	parts := strings.SplitN(entry, "-", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid range format: %q", entry)
	}

	start, err := strconv.Atoi(parts[0])
	if err != nil || start < 0 || start > 4095 {
		return fmt.Errorf("invalid start in range %q: must be int between 0 and 4095", entry)
	}

	count, err := strconv.Atoi(parts[1])
	if err != nil || count < 1 || start+count > 4096 {
		return fmt.Errorf("invalid count in range %q: must be >=1 and start+count <= 4096", entry)
	}

	return nil
}

func isValidPartitionID(entry string) bool {
	id, err := strconv.Atoi(entry)
	return err == nil && id >= 0 && id <= 4095
}

func (r *BackupRoutine) ToModel(
	config *model.BackupConfig,
	nsValidator aerospike.NamespaceValidator,
) (*model.BackupRoutine, error) {
	policy, err := resolveBackupPolicy(r.BackupPolicy, config.BackupPolicies)
	if err != nil {
		return nil, err
	}

	cluster, found := config.AerospikeClusters[r.SourceCluster]
	if !found {
		return nil, errValidationNotFound("Aerospike cluster", r.SourceCluster)
	}

	if cluster.MaxParallelScans != nil {
		if len(r.SetList) > *cluster.MaxParallelScans {
			return nil, fmt.Errorf("max parallel scans must be at least the cardinality of set-list")
		}
	}

	storage, found := config.Storage[r.Storage]
	if !found {
		return nil, errValidationNotFound("storage", r.Storage)
	}

	var secretAgent *model.SecretAgent
	if r.SecretAgent != nil {
		secretAgent, found = config.SecretAgents[*r.SecretAgent]
		if !found {
			return nil, errValidationNotFound("secret agent", *r.SecretAgent)
		}
	}

	missingNSs := nsValidator.MissingNamespaces(cluster, *r.Namespaces)
	if len(missingNSs) > 0 {
		return nil, fmt.Errorf("the following namespaces are missing in the cluster: %v", missingNSs)
	}

	return &model.BackupRoutine{
		BackupPolicy:     policy,
		SourceCluster:    cluster,
		Storage:          storage,
		SecretAgent:      secretAgent,
		IntervalCron:     r.IntervalCron,
		IncrIntervalCron: r.IncrIntervalCron,
		Namespaces:       *r.Namespaces,
		SetList:          r.SetList,
		BinList:          r.BinList,
		PreferRacks:      r.PreferRacks,
		PartitionList:    r.PartitionList,
		NodeList:         r.NodeList,
		Disabled:         r.Disabled,
	}, nil
}

func resolveBackupPolicy(name string, policies map[string]*model.BackupPolicy) (*model.BackupPolicy, error) {
	if name == "" {
		return &model.BackupPolicy{}, nil
	}

	policy, found := policies[name]
	if !found {
		return nil, errValidationNotFound("backup policy", name)
	}

	return policy, nil
}

// NewRoutineFromReader creates a new BackupRoutine object from a given reader.
func NewRoutineFromReader(r io.Reader, format decoder.SerializationFormat) (*BackupRoutine, error) {
	b := &BackupRoutine{}
	if err := decoder.Deserialize(b, r, format); err != nil {
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
	b.fromModel(m, config.BackupConfigCopy())
	return b
}

func (r *BackupRoutine) fromModel(m *model.BackupRoutine, config *model.BackupConfig) {
	r.BackupPolicy = findKeyByValue(config.BackupPolicies, m.BackupPolicy)
	r.SourceCluster = findKeyByValue(config.AerospikeClusters, m.SourceCluster)
	r.Storage = findStorageKey(config.Storage, m.Storage)
	if m.SecretAgent != nil {
		r.SecretAgent = ptr.String(findKeyByValue(config.SecretAgents, m.SecretAgent))
	}
	r.IntervalCron = m.IntervalCron
	r.IncrIntervalCron = m.IncrIntervalCron
	r.Namespaces = &m.Namespaces
	r.SetList = m.SetList
	r.BinList = m.BinList
	r.PreferRacks = m.PreferRacks
	r.PartitionList = m.PartitionList
	r.NodeList = m.NodeList
	r.Disabled = m.Disabled
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
