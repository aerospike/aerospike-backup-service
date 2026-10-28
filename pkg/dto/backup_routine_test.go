package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePartitionList(t *testing.T) {
	valid := []string{
		"",
		"0,100,4095",
		"0-1,100-50,4095-1",
		"0-4096",
		"0-100,100-100",
		"200,0-100,100",
		"EjRWeJq83vEjRRI0VniavN7xI0U=",
	}
	for _, list := range valid {
		t.Run(list, func(t *testing.T) {
			require.NoError(t, validatePartitionList(list))
		})
	}

	const notAnEntry = "is not a partition ID, <begin>-<count> range or digest"
	tests := []struct {
		name    string
		list    string
		wantErr string
	}{
		{name: "space after comma", list: "0, 100", wantErr: `" 100" ` + notAnEntry},
		{name: "space inside a range", list: "0 -10", wantErr: `"0 -10" ` + notAnEntry},
		{name: "empty entry", list: "100,,200", wantErr: `"" ` + notAnEntry},
		{name: "trailing comma", list: "100,", wantErr: `"" ` + notAnEntry},
		{name: "range with zero count", list: "100-0", wantErr: notAnEntry},
		{name: "double dash", list: "100--200", wantErr: notAnEntry},
		{name: "id out of range", list: "4096", wantErr: `"4096" exceeds the last partition 4095`},
		{name: "range out of range", list: "4095-2", wantErr: `"4095-2" exceeds the last partition 4095`},
		{name: "duplicate id", list: "5,5", wantErr: "partition 5 is selected more than once"},
		{name: "id inside a range", list: "0-10,5", wantErr: "partition 5 is selected more than once"},
		{name: "overlapping ranges", list: "100-50,0-101", wantErr: "partition 100 is selected more than once"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ErrorContains(t, validatePartitionList(tt.list), tt.wantErr)
		})
	}
}

func TestBackupRoutine_Validate_PartitionList(t *testing.T) {
	routine := &BackupRoutine{
		IntervalCron:  "@daily",
		SourceCluster: "cluster",
		Storage:       "storage",
		Namespaces:    &[]NamespaceName{"test"},
		PartitionList: "0, 100",
	}

	err := routine.Validate()

	require.ErrorIs(t, err, errInvalidValue)
	assert.ErrorContains(t, err, `partition-list "0, 100": " 100" is not a partition ID`)
}

func TestBackupRoutine_ToModel_PartitionListParallel(t *testing.T) {
	tests := []struct {
		name     string
		parallel int
		wantErr  string
	}{
		{name: "one entry per worker", parallel: 3},
		{name: "more entries than workers", parallel: 2, wantErr: "3 entries exceed the backup policy parallel 2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			routine := &BackupRoutine{
				BackupPolicy:  "policy",
				SourceCluster: "cluster",
				Storage:       "storage",
				IntervalCron:  "@daily",
				Namespaces:    &[]NamespaceName{"test"},
				PartitionList: "0,1,2",
			}
			config := &model.BackupConfig{
				BackupPolicies:    map[string]*model.BackupPolicy{"policy": {Parallel: &tt.parallel}},
				AerospikeClusters: map[string]*model.AerospikeCluster{"cluster": {}},
				Storage:           map[string]model.Storage{"storage": &model.LocalStorage{}},
			}

			_, err := routine.ToModel(config, "r", model.NewServiceLocation("", nil))

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, errInvalidValue)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestBackupRoutine_ToModel(t *testing.T) {
	routineDTO := &BackupRoutine{
		BackupPolicy:     "policy1",
		SourceCluster:    "cluster1",
		Storage:          "storage1",
		SecretAgent:      "agent1",
		IntervalCron:     "cron",
		IncrIntervalCron: "inc_cron",
		Namespaces:       &[]NamespaceName{"ns1"},
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

	m, err := routineDTO.ToModel(config, "r", model.NewServiceLocation("", nil))
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
	assert.Equal(t, "America/New_York", m.Timezone.ResolvedLocation().String())
}

func TestBackupRoutine_ToModel_BlankTimezoneUsesDefault(t *testing.T) {
	t.Parallel()

	for _, timezone := range []ScheduleTimezone{"", " \t "} {
		routineDTO := &BackupRoutine{
			SourceCluster:    "cluster1",
			Storage:          "storage1",
			IntervalCron:     "cron",
			Namespaces:       &[]NamespaceName{"ns1"},
			ScheduleTimezone: timezone,
		}

		m, err := routineDTO.ToModel(&model.BackupConfig{
			AerospikeClusters: map[string]*model.AerospikeCluster{"cluster1": {}},
			Storage:           map[string]model.Storage{"storage1": &model.LocalStorage{}},
		}, "r", model.NewServiceLocation("", nil))

		require.NoError(t, err)
		assert.Equal(t, model.DefaultScheduleTimezone, m.Timezone.ResolvedLocation())
		assert.False(t, m.Timezone.IsExplicit())
		assert.Equal(t, string(timezone), m.Timezone.Configured)
	}
}

func TestBackupRoutine_ToModel_PreservesConfiguredTimezone(t *testing.T) {
	t.Parallel()

	routineDTO := &BackupRoutine{
		SourceCluster:    "cluster1",
		Storage:          "storage1",
		IntervalCron:     "cron",
		Namespaces:       &[]NamespaceName{"ns1"},
		ScheduleTimezone: "utc",
	}

	m, err := routineDTO.ToModel(&model.BackupConfig{
		AerospikeClusters: map[string]*model.AerospikeCluster{"cluster1": {}},
		Storage:           map[string]model.Storage{"storage1": &model.LocalStorage{}},
	}, "r", model.NewServiceLocation("", nil))

	require.NoError(t, err)
	assert.Equal(t, model.DefaultScheduleTimezone, m.Timezone.ResolvedLocation())
	assert.True(t, m.Timezone.IsExplicit())
	assert.Equal(t, "utc", m.Timezone.Configured)
}

func TestBackupRoutine_ToModel_PolicyNotFound(t *testing.T) {
	routineDTO := &BackupRoutine{
		BackupPolicy:  "policy1",
		SourceCluster: "cluster1",
		Namespaces:    &[]NamespaceName{"ns1"},
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

	_, err := routineDTO.ToModel(config, "r", model.NewServiceLocation("", nil))
	require.Error(t, err)
}

func TestBackupRoutine_ToModel_ClusterNotFound(t *testing.T) {
	routineDTO := &BackupRoutine{
		BackupPolicy:  "policy1",
		SourceCluster: "cluster1",
		Storage:       "storage1",
		Namespaces:    &[]NamespaceName{"ns1"},
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

	_, err := routineDTO.ToModel(config, "r", model.NewServiceLocation("", nil))
	require.Error(t, err)
}

func TestBackupRoutine_ToModel_StorageNotFound(t *testing.T) {
	routineDTO := &BackupRoutine{
		BackupPolicy:  "policy1",
		SourceCluster: "cluster1",
		Storage:       "storage1",
		Namespaces:    &[]NamespaceName{"ns1"},
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

	_, err := routineDTO.ToModel(config, "r", model.NewServiceLocation("", nil))
	require.Error(t, err)
}

func TestBackupRoutine_ToModel_SecretAgentNotFound(t *testing.T) {
	routineDTO := &BackupRoutine{
		BackupPolicy:  "policy1",
		SourceCluster: "cluster1",
		Storage:       "storage1",
		SecretAgent:   "agent1",
		Namespaces:    &[]NamespaceName{"ns1"},
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

	_, err := routineDTO.ToModel(config, "r", model.NewServiceLocation("", nil))
	require.Error(t, err)
}

func TestBackupRoutine_Validate_MutualExclusive_RackAndPartition(t *testing.T) {
	r := &BackupRoutine{
		SourceCluster: "cluster1",
		Storage:       "storage1",
		IntervalCron:  "0 0 * * * *",
		Namespaces:    &[]NamespaceName{"ns1"},
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
		Namespaces:    &[]NamespaceName{"ns1"},
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
		Namespaces:    &[]NamespaceName{"ns1"},
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
		Namespaces:    &[]NamespaceName{"ns1"},
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

	_, err := routineDTO.ToModel(config, "r", model.NewServiceLocation("", nil))
	require.Error(t, err)
}

func TestBackupRoutine_ToModel_PreferRacks_ConflictsWithNodeList(t *testing.T) {
	routineDTO := &BackupRoutine{
		BackupPolicy:  "policy1",
		SourceCluster: "cluster1",
		Storage:       "storage1",
		Namespaces:    &[]NamespaceName{"ns1"},
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

	_, err := routineDTO.ToModel(config, "r", model.NewServiceLocation("", nil))
	require.Error(t, err)
}

func TestBackupRoutine_ToModel_ParallelExceedsClusterMax(t *testing.T) {
	routineDTO := &BackupRoutine{
		BackupPolicy:  "policy1",
		SourceCluster: "cluster1",
		Storage:       "storage1",
		Namespaces:    &[]NamespaceName{"ns1"},
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

	_, err := routineDTO.ToModel(config, "r", model.NewServiceLocation("", nil))
	require.ErrorContains(t, err, "backup policy parallelism 8 exceeds cluster max parallelism 4")
}

func TestBackupRoutine_ToModel_ParallelWithinClusterMax(t *testing.T) {
	routineDTO := &BackupRoutine{
		BackupPolicy:  "policy1",
		SourceCluster: "cluster1",
		Storage:       "storage1",
		Namespaces:    &[]NamespaceName{"ns1"},
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

	_, err := routineDTO.ToModel(config, "r", model.NewServiceLocation("", nil))
	require.NoError(t, err)
}

func TestBackupRoutine_ToModel_ParallelEqualsClusterMax(t *testing.T) {
	routineDTO := &BackupRoutine{
		BackupPolicy:  "policy1",
		SourceCluster: "cluster1",
		Storage:       "storage1",
		Namespaces:    &[]NamespaceName{"ns1"},
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

	_, err := routineDTO.ToModel(config, "r", model.NewServiceLocation("", nil))
	require.NoError(t, err)
}

func TestBackupRoutine_ToModel_ParallelUncheckedWhenClusterMaxUnset(t *testing.T) {
	routineDTO := &BackupRoutine{
		BackupPolicy:  "policy1",
		SourceCluster: "cluster1",
		Storage:       "storage1",
		Namespaces:    &[]NamespaceName{"ns1"},
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

	_, err := routineDTO.ToModel(config, "r", model.NewServiceLocation("", nil))
	require.NoError(t, err)
}

func TestBackupRoutine_ToModel_ParallelUncheckedWhenPolicyParallelUnset(t *testing.T) {
	routineDTO := &BackupRoutine{
		BackupPolicy:  "policy1",
		SourceCluster: "cluster1",
		Storage:       "storage1",
		Namespaces:    &[]NamespaceName{"ns1"},
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

	_, err := routineDTO.ToModel(config, "r", model.NewServiceLocation("", nil))
	require.NoError(t, err)
}

func TestBackupRoutine_Validate_InvalidFilterExpression(t *testing.T) {
	r := &BackupRoutine{
		SourceCluster:    "cluster1",
		Storage:          "storage1",
		IntervalCron:     "0 0 * * * *",
		Namespaces:       &[]NamespaceName{"ns1"},
		FilterExpression: "invalid-exp",
	}

	err := r.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse filter expression")
}

func TestBackupRoutine_Validate_RejectsStructurallyInvalidFilterExpression(t *testing.T) {
	t.Skip("BKRS-438: filter-exp is only checked for valid base64, not for a real expression tree")

	r := &BackupRoutine{
		SourceCluster:    "cluster1",
		Storage:          "storage1",
		IntervalCron:     "0 0 * * * *",
		Namespaces:       &[]NamespaceName{"ns1"},
		FilterExpression: "/////////////////////w==",
	}

	err := r.Validate()
	require.Error(t, err)
}

func TestBackupRoutine_Validate_FilterExpressionWithMultipleSets(t *testing.T) {
	r := &BackupRoutine{
		SourceCluster:    "cluster1",
		Storage:          "storage1",
		IntervalCron:     "0 0 * * * *",
		Namespaces:       &[]NamespaceName{"ns1"},
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
		Namespaces:    &[]NamespaceName{"ns1"},
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

	_, err := routineDTO.ToModel(config, "r", model.NewServiceLocation("", nil))
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
		Namespaces:       &[]NamespaceName{"ns1"},
		ScheduleTimezone: "America/New_York",
	}
	require.NoError(t, valid.Validate())

	invalid := &BackupRoutine{
		SourceCluster:    "cluster1",
		Storage:          "storage1",
		IntervalCron:     "@daily",
		Namespaces:       &[]NamespaceName{"ns1"},
		ScheduleTimezone: "Not/AZone",
	}
	require.ErrorContains(t, invalid.Validate(), "Not/AZone")
}
