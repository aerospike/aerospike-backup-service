package dto

import (
	"strings"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClusterFromReader(t *testing.T) {
	yamlCluster := `
seed-nodes:
  - host-name: localhost
    port: 3000
`
	cluster, err := NewClusterFromReader(strings.NewReader(yamlCluster), decoder.YAML)
	require.NoError(t, err)
	require.Len(t, cluster.SeedNodes, 1)
	assert.Equal(t, "localhost", cluster.SeedNodes[0].HostName)
	assert.Equal(t, Port(3000), cluster.SeedNodes[0].Port)
}

func TestNewBackupPolicyFromReader(t *testing.T) {
	yamlPolicy := `
parallel: 4
`
	policy, err := NewBackupPolicyFromReader(strings.NewReader(yamlPolicy), decoder.YAML)
	require.NoError(t, err)
	require.NotNil(t, policy.Parallel)
	assert.Equal(t, 4, *policy.Parallel)
}

func TestNewRoutineFromReader(t *testing.T) {
	yamlRoutine := `
source-cluster: cluster1
storage: storage1
interval-cron: "@daily"
namespaces: []
`
	routine, err := NewRoutineFromReader(strings.NewReader(yamlRoutine), decoder.YAML)
	require.NoError(t, err)
	assert.Equal(t, "cluster1", routine.SourceCluster)
	assert.Equal(t, "storage1", routine.Storage)
	assert.Equal(t, "@daily", routine.IntervalCron)
	require.NotNil(t, routine.Namespaces)
	assert.Empty(t, *routine.Namespaces)
}

func TestNewBackupDetailsFromModel(t *testing.T) {
	created := time.Date(2025, 3, 20, 14, 50, 0, 0, time.UTC)
	finished := created.Add(30 * time.Second)
	from := created.Add(-24 * time.Hour)
	storage := &model.LocalStorage{Path: "/tmp/backups"}
	detailsModel := model.NewBackupDetails(
		model.BackupMetadata{
			Created:             created,
			Finished:            finished,
			From:                from,
			Namespace:           "test-ns",
			RecordCount:         100,
			ByteCount:           2000,
			FileCount:           1,
			SecondaryIndexCount: 5,
			UDFCount:            2,
			Compression:         model.CompressionModeNone,
			Encryption:          model.EncryptionModeNone,
		},
		"daily/backup/key",
		storage,
	)

	config := &model.BackupConfig{
		Storage: map[string]model.Storage{
			"storage1": storage,
		},
	}

	details := NewBackupDetailsFromModel(&detailsModel, config)
	require.NotNil(t, details)
	assert.Equal(t, "daily/backup/key", details.Key)
	assert.Equal(t, created, details.Created)
	assert.Equal(t, created.UnixMilli(), details.Timestamp)
	assert.Equal(t, finished, details.Finished)
	assert.Equal(t, finished.UnixMilli(), details.FinishedTimestamp)
	assert.Equal(t, uint(30), details.Duration)
	assert.Equal(t, from, details.From)
	assert.Equal(t, "test-ns", details.Namespace)
	assert.Equal(t, uint64(100), details.RecordCount)
	require.NotNil(t, details.Storage)
}

func TestBackupCommonConfig_fromModel(t *testing.T) {
	format := model.TimestampFormatISO
	timezone, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	var dtoConfig BackupCommonConfig
	dtoConfig.fromModel(&model.BackupCommonConfig{
		TimestampFormat:    &format,
		Timezone:           timezone,
		ConfiguredTimezone: "America/New_York",
	})

	assert.Equal(t, TimestampFormatISO, dtoConfig.TimestampFormat)
	assert.Equal(t, "America/New_York", dtoConfig.ScheduleTimezone)

	roundTrip := dtoConfig.ToModel()
	require.NotNil(t, roundTrip.TimestampFormat)
	assert.Equal(t, model.TimestampFormatISO, *roundTrip.TimestampFormat)
	assert.Equal(t, timezone.String(), roundTrip.Timezone.String())
	assert.Equal(t, "America/New_York", roundTrip.ConfiguredTimezone)
}

func TestNewRoutineFromModel(t *testing.T) {
	policy := &model.BackupPolicy{}
	cluster := &model.AerospikeCluster{}
	storage := &model.LocalStorage{Path: "/tmp"}

	config := model.NewConfig()
	require.NoError(t, config.AddPolicy("policy1", policy))
	require.NoError(t, config.AddCluster("cluster1", cluster))
	require.NoError(t, config.AddStorage("storage1", storage))

	routineModel := &model.BackupRoutine{
		BackupPolicy:  policy,
		SourceCluster: cluster,
		Storage:       storage,
		IntervalCron:  "@hourly",
		Namespaces:    []string{"ns1"},
		Disabled:      true,
	}

	routine := NewRoutineFromModel(routineModel, config)
	require.NotNil(t, routine)
	assert.Equal(t, "policy1", routine.BackupPolicy)
	assert.Equal(t, "cluster1", routine.SourceCluster)
	assert.Equal(t, "storage1", routine.Storage)
	assert.Equal(t, "@hourly", routine.IntervalCron)
	require.NotNil(t, routine.Namespaces)
	assert.Equal(t, []string{"ns1"}, *routine.Namespaces)
	assert.True(t, routine.Disabled)
}

func TestNewRoutineFromModel_ScheduleTimezone(t *testing.T) {
	policy := &model.BackupPolicy{}
	cluster := &model.AerospikeCluster{}
	storage := &model.LocalStorage{Path: "/tmp"}

	config := model.NewConfig()
	timezone, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	config.ServiceConfig.Backup = &model.BackupCommonConfig{
		Timezone:           timezone,
		ConfiguredTimezone: "America/New_York",
	}
	require.NoError(t, config.AddPolicy("policy1", policy))
	require.NoError(t, config.AddCluster("cluster1", cluster))
	require.NoError(t, config.AddStorage("storage1", storage))

	newRoutine := func(configured string, resolved *time.Location) *model.BackupRoutine {
		return &model.BackupRoutine{
			BackupPolicy:       policy,
			SourceCluster:      cluster,
			Storage:            storage,
			IntervalCron:       "@hourly",
			Namespaces:         []string{"ns1"},
			Timezone:           resolved,
			ConfiguredTimezone: configured,
		}
	}

	ny := config.ServiceConfig.Backup.GetTimezoneOrDefault()

	t.Run("inherited value is omitted", func(t *testing.T) {
		routine := NewRoutineFromModel(newRoutine("", ny), config)
		require.NotNil(t, routine)
		assert.Empty(t, routine.ScheduleTimezone)
	})

	t.Run("explicit value differing from default is kept", func(t *testing.T) {
		routine := NewRoutineFromModel(newRoutine("UTC", time.UTC), config)
		require.NotNil(t, routine)
		assert.Equal(t, "UTC", routine.ScheduleTimezone)
	})

	t.Run("explicit value matching default is kept", func(t *testing.T) {
		routine := NewRoutineFromModel(newRoutine("America/New_York", ny), config)
		require.NotNil(t, routine)
		assert.Equal(t, "America/New_York", routine.ScheduleTimezone)
	})

	t.Run("configured keyword is preserved", func(t *testing.T) {
		routine := NewRoutineFromModel(newRoutine("utc", time.UTC), config)
		require.NotNil(t, routine)
		assert.Equal(t, "utc", routine.ScheduleTimezone)
	})
}

func TestNewClusterFromReader_InvalidYAML(t *testing.T) {
	_, err := NewClusterFromReader(strings.NewReader("seed-nodes: []"), decoder.YAML)
	require.Error(t, err)
}

func TestNewBackupDetailsFromModel_Nil(t *testing.T) {
	assert.Nil(t, NewBackupDetailsFromModel(nil, &model.BackupConfig{}))
}
