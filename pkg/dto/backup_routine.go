package dto

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/aws/smithy-go/ptr"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/reugn/go-quartz/quartz"
)

const maxChunks = 10_000

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
	// The list of namespaces to back up.
	// If empty, the entire cluster is backed up.
	// The order of namespaces does not determine the backup execution or completion order.
	Namespaces *[]string `yaml:"namespaces,omitempty" json:"namespaces,omitempty" example:"[\"source-ns1\"]" validate:"required"`
	// The list of backup set names (optional, an empty list implies backing up all sets).
	SetList []string `yaml:"set-list,omitempty" json:"set-list,omitempty" example:"set1" extensions:"x-nullable"`
	// The list of backup bin names (optional, an empty list implies backing up all bins) extensions:"x-nullable".
	BinList []string `yaml:"bin-list,omitempty" json:"bin-list,omitempty" example:"dataBin" extensions:"x-nullable"`
	// RackList specifies the Aerospike Server rack IDs from which to read records
	// during backup.
	// If provided, only nodes belonging to these specified racks will be scanned.
	// If the list is empty or omitted, no rack filtering is applied.
	// Mutually exclusive with partition-list and node-list in this routine,
	// and also mutually exclusive with the cluster's prefer-racks setting.
	RackList []int `yaml:"rack-list,omitempty" json:"rack-list,omitempty" extensions:"x-nullable"`

	// PartitionList defines the list of partitions to include in the backup.
	// The format supports individual partitions or ranges.
	// - A range is specified as "<start>-<count>" (e.g., "100-50" backs up 50 partitions starting from 100).
	// - A single partition is specified as a number (e.g., "0").
	// Multiple entries can be comma-separated: e.g., "0,100,200,300,400,500".
	// By default, all partitions (0 to 4095) are backed up.
	// Mutually exclusive with node-list and rack-list in this routine,
	// and also mutually exclusive with the cluster's prefer-racks setting.
	PartitionList string `yaml:"partition-list,omitempty" json:"partition-list,omitempty" extensions:"x-nullable"`

	// NodeList specifies which Aerospike nodes to include in the backup.
	// Only the listed nodes will be backed up.
	// Each node can be specified as one of the following:
	// - "<IP address>:<port>"
	// - "<hostname>:<port>"
	// - "<node ID>"
	// To obtain node identifiers, run: `asinfo -v "service:"`.
	// If using IP addresses or hostnames, ensure they match the values returned by the `asinfo` command.
	// Mutually exclusive with partition-list and rack-list in this routine,
	// and also mutually exclusive with the cluster's prefer-racks setting.
	// Parallelism is determined by the number of listed nodes unless `BackupPolicy.Parallel` is set to a lower value.
	NodeList []string `yaml:"node-list,omitempty" json:"node-list,omitempty" extensions:"x-nullable"`

	// Base64 encoded filter expression. Use the encoded filter expression in each scan call,
	// which can be used to do a partial backup. The expression to be used can be Base64
	// encoded through any client. This argument is mutually exclusive with multi-set backup.
	FilterExpression string `yaml:"filter-exp,omitempty" json:"filter-exp,omitempty" extensions:"x-nullable"`

	// Whether this routine is disabled and should not run. Default: false.
	Disabled bool `json:"disabled,omitempty" yaml:"disabled,omitempty" default:"false"`
}

const (
	partitionListField = "partition-list"
	rackListField      = "rack-list"
	nodeListField      = "node-list"
	preferRacksField   = "prefer-racks"
)

// Validate validates the backup routine configuration.
//
//nolint:gocognit,funlen
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
	for i, rack := range r.RackList {
		if rack < 0 {
			return errValidationNegative(fmt.Sprintf("rack-list[%d]", i), rack)
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
	// Mutual exclusivity within routine: rack-list, partition-list, node-list
	if len(r.PartitionList) > 0 && len(r.NodeList) > 0 {
		return errValidationMutuallyExclusive(partitionListField, nodeListField)
	}
	if len(r.RackList) > 0 && len(r.PartitionList) > 0 {
		return errValidationMutuallyExclusive(rackListField, partitionListField)
	}
	if len(r.RackList) > 0 && len(r.NodeList) > 0 {
		return errValidationMutuallyExclusive(rackListField, nodeListField)
	}
	if r.Namespaces == nil {
		return errValidationEmptyField("namespaces")
	}
	for i, ns := range *r.Namespaces {
		if ns == "" {
			return errValidationEmptyField(fmt.Sprintf("namespaces[%d]", i))
		}
	}

	if duplicates := collections.CheckDuplicates(*r.Namespaces); len(duplicates) > 0 {
		return errValidationDuplicate("namespaces", duplicates)
	}
	if duplicates := collections.CheckDuplicates(r.SetList); len(duplicates) > 0 {
		return errValidationDuplicate("set-list", duplicates)
	}
	for i, set := range r.SetList {
		if set == "" {
			return errValidationEmptyField(fmt.Sprintf("set-list[%d]", i))
		}
	}
	if duplicates := collections.CheckDuplicates(r.BinList); len(duplicates) > 0 {
		return errValidationDuplicate("bin-list", duplicates)
	}
	for i, bin := range r.BinList {
		if bin == "" {
			return errValidationEmptyField(fmt.Sprintf("bin-list[%d]", i))
		}
	}
	if duplicates := collections.CheckDuplicates(r.RackList); len(duplicates) > 0 {
		return errValidationDuplicate(rackListField, duplicates)
	}
	if duplicates := collections.CheckDuplicates(r.NodeList); len(duplicates) > 0 {
		return errValidationDuplicate(nodeListField, duplicates)
	}
	if r.FilterExpression != "" {
		if len(r.SetList) > 1 {
			return fmt.Errorf("filter-exp cannot be used when backing up multiple sets")
		}
		if _, err := as.ExpFromBase64(r.FilterExpression); err != nil {
			return fmt.Errorf("failed to parse filter expression: %w", err)
		}
	}

	return nil
}

func validatePartitionList(partitionList string) error {
	if partitionList == "" {
		return nil // empty list is valid
	}

	for entry := range strings.SplitSeq(partitionList, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			return errors.New("empty entry in partition list")
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

func (r *BackupRoutine) ToModel(config *model.BackupConfig, name string) (*model.BackupRoutine, error) {
	policy, err := resolveBackupPolicy(r.BackupPolicy, config.BackupPolicies)
	if err != nil {
		return nil, err
	}

	cluster, found := config.AerospikeClusters[r.SourceCluster]
	if !found {
		return nil, errValidationNotFound("Aerospike cluster", r.SourceCluster)
	}

	// Enforce mutual exclusivity between routine-level selectors and cluster-level prefer-racks
	if len(cluster.PreferRacks) > 0 {
		if len(r.RackList) > 0 {
			return nil, errValidationMutuallyExclusive(rackListField, preferRacksField)
		}
		if len(r.PartitionList) > 0 {
			return nil, errValidationMutuallyExclusive(partitionListField, preferRacksField)
		}
		if len(r.NodeList) > 0 {
			return nil, errValidationMutuallyExclusive(nodeListField, preferRacksField)
		}
	}

	if err := ValidateBackupPolicyParallelism(policy, cluster); err != nil {
		return nil, err
	}

	storage, found := config.Storage[r.Storage]
	if !found {
		return nil, errValidationNotFound("storage", r.Storage)
	}

	if err = validateFileLimit(policy, storage); err != nil {
		return nil, err
	}

	var secretAgent *model.SecretAgent
	if r.SecretAgent != nil {
		secretAgent, found = config.SecretAgents[*r.SecretAgent]
		if !found {
			return nil, errValidationNotFound("secret agent", *r.SecretAgent)
		}
	}

	return &model.BackupRoutine{
		Name:             name,
		BackupPolicy:     policy,
		SourceCluster:    cluster,
		Storage:          storage,
		SecretAgent:      secretAgent,
		IntervalCron:     r.IntervalCron,
		IncrIntervalCron: r.IncrIntervalCron,
		Namespaces:       *r.Namespaces,
		SetList:          r.SetList,
		BinList:          r.BinList,
		RackList:         r.RackList,
		PartitionList:    r.PartitionList,
		NodeList:         r.NodeList,
		FilterExpression: r.FilterExpression,
		Disabled:         r.Disabled,
	}, nil
}

func validateFileLimit(policy *model.BackupPolicy, storage model.Storage) error {
	if storage == nil || policy == nil {
		return nil
	}

	// Skip this check for the local storage.
	if _, ok := storage.(*model.LocalStorage); ok {
		return nil
	}

	// Attention! Part szie is in bytes, but file limit is in MB.
	partSize := storage.GetPartSizeOrDefault()
	fileLimit := policy.GetFileLimitOrDefault() * 1024 * 1024

	if partSize > 0 {
		// integer implementation of ceiling division.
		chunks := (fileLimit-1)/(partSize) + 1
		if chunks >= maxChunks {
			return fmt.Errorf(
				"file limit %d with chunk size %d requires %d chunks, exceeds maximum of %d: "+
					"increase chunk size or decrease file limit",
				fileLimit, partSize, chunks, maxChunks,
			)
		}
	}

	return nil
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
	r.RackList = m.RackList
	r.PartitionList = m.PartitionList
	r.NodeList = m.NodeList
	r.FilterExpression = m.FilterExpression
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
