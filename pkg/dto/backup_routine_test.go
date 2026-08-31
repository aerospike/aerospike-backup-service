package dto

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
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
		SecretAgent:      "agent1",
		IntervalCron:     "cron",
		IncrIntervalCron: "inc_cron",
		Namespaces:       &[]string{"ns1"},
		SetList:          []string{"set1"},
		BinList:          []string{"bin1"},
		RackList:         []int{1},
		PartitionList:    "0-100",
		NodeList:         []string{"node1"},
		FilterExpression: "k1EDpHRlc3Q=",
		Disabled:         true,
		ScheduleTimezone: "America/New_York",
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
	assert.Equal(t, "k1EDpHRlc3Q=", m.FilterExpression)
	assert.True(t, m.Disabled)
	assert.Equal(t, "America/New_York", m.Timezone.String())
}

func TestBackupRoutine_ToModel_BlankTimezoneIsNil(t *testing.T) {
	t.Parallel()

	for _, timezone := range []string{"", " \t "} {
		routineDTO := &BackupRoutine{
			SourceCluster:    "cluster1",
			Storage:          "storage1",
			IntervalCron:     "cron",
			Namespaces:       &[]string{"ns1"},
			ScheduleTimezone: timezone,
		}

		m, err := routineDTO.ToModel(&model.BackupConfig{
			AerospikeClusters: map[string]*model.AerospikeCluster{"cluster1": {}},
			Storage:           map[string]model.Storage{"storage1": &model.LocalStorage{}},
		}, "r")
		require.NoError(t, err)
		assert.Nil(t, m.Timezone)
		assert.Empty(t, m.ConfiguredTimezone)
	}
}

func TestBackupRoutine_ToModel_CanonicalizesConfiguredTimezone(t *testing.T) {
	t.Parallel()

	routineDTO := &BackupRoutine{
		SourceCluster:    "cluster1",
		Storage:          "storage1",
		IntervalCron:     "cron",
		Namespaces:       &[]string{"ns1"},
		ScheduleTimezone: "utc",
	}

	m, err := routineDTO.ToModel(&model.BackupConfig{
		AerospikeClusters: map[string]*model.AerospikeCluster{"cluster1": {}},
		Storage:           map[string]model.Storage{"storage1": &model.LocalStorage{}},
	}, "r")
	require.NoError(t, err)
	assert.Equal(t, time.UTC, m.Timezone)
	assert.Equal(t, "UTC", m.ConfiguredTimezone)
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
		SecretAgent:   "agent1",
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

func TestBackupRoutine_ToModel_ParallelExceedsClusterMax(t *testing.T) {
	routineDTO := &BackupRoutine{
		BackupPolicy:  "policy1",
		SourceCluster: "cluster1",
		Storage:       "storage1",
		Namespaces:    &[]string{"ns1"},
	}

	config := &model.BackupConfig{
		BackupPolicies: map[string]*model.BackupPolicy{
			"policy1": {Parallel: ptr.Of(8)},
		},
		AerospikeClusters: map[string]*model.AerospikeCluster{
			"cluster1": {MaxParallelScans: ptr.Of(4)},
		},
		Storage: map[string]model.Storage{
			"storage1": &model.S3Storage{},
		},
	}

	_, err := routineDTO.ToModel(config, "r")
	require.ErrorContains(t, err, "backup policy parallelism 8 exceeds cluster max parallelism 4")
}

func TestBackupRoutine_ToModel_ParallelWithinClusterMax(t *testing.T) {
	routineDTO := &BackupRoutine{
		BackupPolicy:  "policy1",
		SourceCluster: "cluster1",
		Storage:       "storage1",
		Namespaces:    &[]string{"ns1"},
	}

	config := &model.BackupConfig{
		BackupPolicies: map[string]*model.BackupPolicy{
			"policy1": {Parallel: ptr.Of(4)},
		},
		AerospikeClusters: map[string]*model.AerospikeCluster{
			"cluster1": {MaxParallelScans: ptr.Of(8)},
		},
		Storage: map[string]model.Storage{
			"storage1": &model.S3Storage{},
		},
	}

	_, err := routineDTO.ToModel(config, "r")
	require.NoError(t, err)
}

func TestBackupRoutine_ToModel_ParallelEqualsClusterMax(t *testing.T) {
	routineDTO := &BackupRoutine{
		BackupPolicy:  "policy1",
		SourceCluster: "cluster1",
		Storage:       "storage1",
		Namespaces:    &[]string{"ns1"},
	}

	config := &model.BackupConfig{
		BackupPolicies: map[string]*model.BackupPolicy{
			"policy1": {Parallel: ptr.Of(4)},
		},
		AerospikeClusters: map[string]*model.AerospikeCluster{
			"cluster1": {MaxParallelScans: ptr.Of(4)},
		},
		Storage: map[string]model.Storage{
			"storage1": &model.S3Storage{},
		},
	}

	_, err := routineDTO.ToModel(config, "r")
	require.NoError(t, err)
}

func TestBackupRoutine_ToModel_ParallelUncheckedWhenClusterMaxUnset(t *testing.T) {
	routineDTO := &BackupRoutine{
		BackupPolicy:  "policy1",
		SourceCluster: "cluster1",
		Storage:       "storage1",
		Namespaces:    &[]string{"ns1"},
	}

	config := &model.BackupConfig{
		BackupPolicies: map[string]*model.BackupPolicy{
			"policy1": {Parallel: ptr.Of(100)},
		},
		AerospikeClusters: map[string]*model.AerospikeCluster{
			"cluster1": {},
		},
		Storage: map[string]model.Storage{
			"storage1": &model.S3Storage{},
		},
	}

	_, err := routineDTO.ToModel(config, "r")
	require.NoError(t, err)
}

func TestBackupRoutine_ToModel_ParallelUncheckedWhenPolicyParallelUnset(t *testing.T) {
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
			"cluster1": {MaxParallelScans: ptr.Of(4)},
		},
		Storage: map[string]model.Storage{
			"storage1": &model.S3Storage{},
		},
	}

	_, err := routineDTO.ToModel(config, "r")
	require.NoError(t, err)
}

func TestBackupRoutine_Validate_InvalidFilterExpression(t *testing.T) {
	r := &BackupRoutine{
		SourceCluster:    "cluster1",
		Storage:          "storage1",
		IntervalCron:     "0 0 * * * *",
		Namespaces:       &[]string{"ns1"},
		FilterExpression: "invalid-exp",
	}

	err := r.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse filter expression")
}

func TestBackupRoutine_Validate_FilterExpressionWithMultipleSets(t *testing.T) {
	r := &BackupRoutine{
		SourceCluster:    "cluster1",
		Storage:          "storage1",
		IntervalCron:     "0 0 * * * *",
		Namespaces:       &[]string{"ns1"},
		SetList:          []string{"set1", "set2"},
		FilterExpression: "k1EDpHRlc3Q=",
	}

	err := r.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "filter-exp cannot be used when backing up multiple sets")
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

const defaultPartSize = 50 * 1024 * 1024

type mockStorage struct {
	partSize *int
}

func (m *mockStorage) GetPath() string                     { return "" }
func (m *mockStorage) GetStorageClass() model.StorageClass { return model.StorageClass{} }
func (m *mockStorage) String() string                      { return "" }
func (m *mockStorage) GetPartSizeOrDefault() int {
	if m.partSize != nil {
		return *m.partSize
	}

	return defaultPartSize
}

func TestValidateFileLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		policy  *model.BackupPolicy
		storage model.Storage
		wantErr string
	}{
		{
			name:    "nil storage returns nil",
			policy:  &model.BackupPolicy{FileLimit: ptr.Of(100 * 1024 * 1024)},
			storage: nil,
		},
		{
			name:    "nil policy returns nil",
			policy:  nil,
			storage: &mockStorage{partSize: ptr.Of(100 * 1024 * 1024)},
		},
		{
			name:    "nil file limit defaults to 250MB, nil part size defaults to 50MB",
			policy:  &model.BackupPolicy{FileLimit: nil},
			storage: &mockStorage{partSize: nil},
			// fileLimit = 250MB, partSize = 50MB, chunks = ceil(250/50) = 5
		},
		{
			name:    "nil file limit defaults to 250MB with explicit part size",
			policy:  &model.BackupPolicy{FileLimit: nil},
			storage: &mockStorage{partSize: ptr.Of(100 * 1024 * 1024)},
			// fileLimit = 250MB, chunks = ceil(250MB/100) = 2621440
		},
		{
			name:    "valid: single chunk",
			policy:  &model.BackupPolicy{FileLimit: ptr.Of(1)},
			storage: &mockStorage{partSize: ptr.Of(defaultPartSize)},
			// fileLimit = 1MB, partSize = 50MB, chunks = 1
		},
		{
			name:    "valid: exact chunk boundary",
			policy:  &model.BackupPolicy{FileLimit: ptr.Of(500)},
			storage: &mockStorage{partSize: ptr.Of(defaultPartSize)},
			// fileLimit = 500MB, partSize = 50MB, chunks = 10
		},
		{
			name:    "valid: non-divisible rounds up",
			policy:  &model.BackupPolicy{FileLimit: ptr.Of(51)},
			storage: &mockStorage{partSize: ptr.Of(defaultPartSize)},
			// fileLimit = 51MB, partSize = 50MB, chunks = ceil(51MB/50MB) = 2
		},
		{
			name:    "valid: just below max chunks",
			policy:  &model.BackupPolicy{FileLimit: ptr.Of(maxChunks - 1)},
			storage: &mockStorage{partSize: ptr.Of(1 * 1024 * 1024)},
			// fileLimit = 9999MB, partSize = 1MB, chunks = 9999
		},
		{
			name:    "invalid: exactly max chunks",
			policy:  &model.BackupPolicy{FileLimit: ptr.Of(maxChunks)},
			storage: &mockStorage{partSize: ptr.Of(1 * 1024 * 1024)},
			// fileLimit = 10000MB, partSize = 1MB, chunks = 10000
			wantErr: "exceeds maximum of 10000",
		},
		{
			name:    "invalid: exceeds max chunks",
			policy:  &model.BackupPolicy{FileLimit: ptr.Of(maxChunks + 1)},
			storage: &mockStorage{partSize: ptr.Of(1 * 1024 * 1024)},
			// fileLimit = 10001MB, partSize = 1MB, chunks = 10001
			wantErr: "exceeds maximum of 10000",
		},
		{
			name:    "invalid: nil part size defaults to 50MB, file limit exceeds max chunks",
			policy:  &model.BackupPolicy{FileLimit: ptr.Of(maxChunks * 50)},
			storage: &mockStorage{partSize: nil},
			// fileLimit = 500000MB, partSize = 50MB, chunks = 10000
			wantErr: "exceeds maximum of 10000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateFileLimit(tt.policy, tt.storage)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestBackupRoutine_Validate_ScheduleTimezone(t *testing.T) {
	t.Parallel()

	valid := &BackupRoutine{
		SourceCluster:    "cluster1",
		Storage:          "storage1",
		IntervalCron:     "@daily",
		Namespaces:       &[]string{"ns1"},
		ScheduleTimezone: "America/New_York",
	}
	require.NoError(t, valid.Validate())

	invalid := &BackupRoutine{
		SourceCluster:    "cluster1",
		Storage:          "storage1",
		IntervalCron:     "@daily",
		Namespaces:       &[]string{"ns1"},
		ScheduleTimezone: "EST",
	}
	require.ErrorContains(t, invalid.Validate(), "EST")
}
