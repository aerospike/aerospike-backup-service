package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aws/smithy-go/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidSinglePartitionID(t *testing.T) {
	err := validatePartitionList("0,100,4095")
	require.NoError(t, err)
}

func TestInvalidPartitionID_OutOfRange(t *testing.T) {
	err := validatePartitionList("4096")
	require.Error(t, err)
}

func TestValidPartitionRange(t *testing.T) {
	err := validatePartitionList("0-1,100-50,4095-1")
	require.NoError(t, err)
}

func TestInvalidPartitionRange_StartTooHigh(t *testing.T) {
	err := validatePartitionList("4095-2")
	require.Error(t, err)
}

func TestInvalidPartitionRange_CountZero(t *testing.T) {
	err := validatePartitionList("100-0")
	require.Error(t, err)
}

func TestInvalidPartitionRange_BadFormat(t *testing.T) {
	err := validatePartitionList("100--200")
	require.Error(t, err)
}

func TestEmptyString(t *testing.T) {
	err := validatePartitionList("")
	require.NoError(t, err)
}

func TestEmptyEntry(t *testing.T) {
	err := validatePartitionList("100,,200")
	require.Error(t, err)
}

func TestBackupRoutine_ToModel(t *testing.T) {
	routineDTO := &BackupRoutine{
		BackupPolicy:     "policy1",
		SourceCluster:    "cluster1",
		Storage:          "storage1",
		SecretAgent:      ptr.String("agent1"),
		IntervalCron:     "cron",
		IncrIntervalCron: "inc_cron",
		Namespaces:       &[]string{"ns1"},
		SetList:          []string{"set1"},
		BinList:          []string{"bin1"},
		RackList:         []int{1},
		PartitionList:    "0-100",
		NodeList:         []string{"node1"},
		Disabled:         true,
	}

	config := &model.BackupConfig{
		BackupPolicies: map[string]*model.BackupPolicy{
			"policy1": {},
		},
		AerospikeClusters: map[string]*model.AerospikeCluster{
			"cluster1": {},
		},
		Storage: map[string]model.Storage{
			"storage1": &model.S3Storage{},
		},
		SecretAgents: map[string]*model.SecretAgent{
			"agent1": {},
		},
	}

	m, err := routineDTO.ToModel(config, "r")
	require.NoError(t, err)

	assert.Equal(t, config.BackupPolicies["policy1"], m.BackupPolicy)
	assert.Equal(t, config.AerospikeClusters["cluster1"], m.SourceCluster)
	assert.Equal(t, config.Storage["storage1"], m.Storage)
	assert.Equal(t, config.SecretAgents["agent1"], m.SecretAgent)
	assert.Equal(t, "cron", m.IntervalCron)
	assert.Equal(t, "inc_cron", m.IncrIntervalCron)
	assert.Equal(t, []string{"ns1"}, m.Namespaces)
	assert.Equal(t, []string{"set1"}, m.SetList)
	assert.Equal(t, []string{"bin1"}, m.BinList)
	assert.Equal(t, []int{1}, m.RackList)
	assert.Equal(t, "0-100", m.PartitionList)
	assert.Equal(t, []string{"node1"}, m.NodeList)
	assert.True(t, m.Disabled)
}

func TestBackupRoutine_ToModel_PolicyNotFound(t *testing.T) {
	routineDTO := &BackupRoutine{
		BackupPolicy:  "policy1",
		SourceCluster: "cluster1",
		Namespaces:    &[]string{"ns1"},
	}

	config := &model.BackupConfig{
		BackupPolicies: map[string]*model.BackupPolicy{},
		AerospikeClusters: map[string]*model.AerospikeCluster{
			"cluster1": {},
		},
		Storage: map[string]model.Storage{
			"storage1": &model.S3Storage{},
		},
	}

	_, err := routineDTO.ToModel(config, "r")
	require.Error(t, err)
}

func TestBackupRoutine_ToModel_ClusterNotFound(t *testing.T) {
	routineDTO := &BackupRoutine{
		BackupPolicy:  "policy1",
		SourceCluster: "cluster1",
		Storage:       "storage1",
		Namespaces:    &[]string{"ns1"},
	}

	config := &model.BackupConfig{
		BackupPolicies: map[string]*model.BackupPolicy{
			"policy1": {},
		},
		AerospikeClusters: map[string]*model.AerospikeCluster{},
		Storage: map[string]model.Storage{
			"storage1": &model.S3Storage{},
		},
	}

	_, err := routineDTO.ToModel(config, "r")
	require.Error(t, err)
}

func TestBackupRoutine_ToModel_StorageNotFound(t *testing.T) {
	routineDTO := &BackupRoutine{
		BackupPolicy:  "policy1",
		SourceCluster: "cluster1",
		Storage:       "storage1",
		Namespaces:    &[]string{"ns1"},
	}

	config := &model.BackupConfig{
		BackupPolicies: map[string]*model.BackupPolicy{
			"policy1": {},
		},
		AerospikeClusters: map[string]*model.AerospikeCluster{
			"cluster1": {},
		},
		Storage: map[string]model.Storage{},
	}

	_, err := routineDTO.ToModel(config, "r")
	require.Error(t, err)
}

func TestBackupRoutine_ToModel_SecretAgentNotFound(t *testing.T) {
	routineDTO := &BackupRoutine{
		BackupPolicy:  "policy1",
		SourceCluster: "cluster1",
		Storage:       "storage1",
		SecretAgent:   ptr.String("agent1"),
		Namespaces:    &[]string{"ns1"},
	}

	config := &model.BackupConfig{
		BackupPolicies: map[string]*model.BackupPolicy{
			"policy1": {},
		},
		AerospikeClusters: map[string]*model.AerospikeCluster{
			"cluster1": {},
		},
		Storage: map[string]model.Storage{
			"storage1": &model.S3Storage{},
		},
		SecretAgents: map[string]*model.SecretAgent{},
	}

	_, err := routineDTO.ToModel(config, "r")
	require.Error(t, err)
}

func TestBackupRoutine_Validate_MutualExclusive_RackAndPartition(t *testing.T) {
	r := &BackupRoutine{
		SourceCluster: "cluster1",
		Storage:       "storage1",
		IntervalCron:  "0 0 * * * *",
		Namespaces:    &[]string{"ns1"},
		RackList:      []int{1},
		PartitionList: "0-1",
	}
	require.Error(t, r.Validate())
}

func TestBackupRoutine_Validate_MutualExclusive_RackAndNode(t *testing.T) {
	r := &BackupRoutine{
		SourceCluster: "cluster1",
		Storage:       "storage1",
		IntervalCron:  "0 0 * * * *",
		Namespaces:    &[]string{"ns1"},
		RackList:      []int{1},
		NodeList:      []string{"node1"},
	}
	require.Error(t, r.Validate())
}

func TestBackupRoutine_Validate_MutualExclusive_PartitionAndNode(t *testing.T) {
	r := &BackupRoutine{
		SourceCluster: "cluster1",
		Storage:       "storage1",
		IntervalCron:  "0 0 * * * *",
		Namespaces:    &[]string{"ns1"},
		PartitionList: "0-1",
		NodeList:      []string{"node1"},
	}
	require.Error(t, r.Validate())
}

func TestBackupRoutine_ToModel_PreferRacks_ConflictsWithPartitionList(t *testing.T) {
	routineDTO := &BackupRoutine{
		BackupPolicy:  "policy1",
		SourceCluster: "cluster1",
		Storage:       "storage1",
		Namespaces:    &[]string{"ns1"},
		PartitionList: "0-1",
	}

	config := &model.BackupConfig{
		BackupPolicies: map[string]*model.BackupPolicy{
			"policy1": {},
		},
		AerospikeClusters: map[string]*model.AerospikeCluster{
			"cluster1": {PreferRacks: []int{2}},
		},
		Storage: map[string]model.Storage{
			"storage1": &model.S3Storage{},
		},
	}

	_, err := routineDTO.ToModel(config, "r")
	require.Error(t, err)
}

func TestBackupRoutine_ToModel_PreferRacks_ConflictsWithNodeList(t *testing.T) {
	routineDTO := &BackupRoutine{
		BackupPolicy:  "policy1",
		SourceCluster: "cluster1",
		Storage:       "storage1",
		Namespaces:    &[]string{"ns1"},
		NodeList:      []string{"node1"},
	}

	config := &model.BackupConfig{
		BackupPolicies: map[string]*model.BackupPolicy{
			"policy1": {},
		},
		AerospikeClusters: map[string]*model.AerospikeCluster{
			"cluster1": {PreferRacks: []int{2}},
		},
		Storage: map[string]model.Storage{
			"storage1": &model.S3Storage{},
		},
	}

	_, err := routineDTO.ToModel(config, "r")
	require.Error(t, err)
}

func TestBackupRoutine_ToModel_PreferRacks_ConflictsWithRackList(t *testing.T) {
	routineDTO := &BackupRoutine{
		BackupPolicy:  "policy1",
		SourceCluster: "cluster1",
		Storage:       "storage1",
		RackList:      []int{1},
		Namespaces:    &[]string{"ns1"},
	}

	config := &model.BackupConfig{
		BackupPolicies: map[string]*model.BackupPolicy{
			"policy1": {},
		},
		AerospikeClusters: map[string]*model.AerospikeCluster{
			"cluster1": {PreferRacks: []int{2}},
		},
		Storage: map[string]model.Storage{
			"storage1": &model.S3Storage{},
		},
	}

	_, err := routineDTO.ToModel(config, "r")
	require.Error(t, err)
}
