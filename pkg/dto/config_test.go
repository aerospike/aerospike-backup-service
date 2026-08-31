package dto

import (
	"strings"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validConfig() *Config {
	return &Config{
		ServiceConfig: ServiceConfig{},
		BackupRoutines: map[string]*BackupRoutine{
			"routine1": {
				SourceCluster: "cluster1",
				BackupPolicy:  "policy1",
				Storage:       "storage1",
				Namespaces:    ptr.Of([]string{"ns1"}),
				IntervalCron:  "* * * * * *",
			},
			"routine2": {
				SourceCluster: "cluster2",
				BackupPolicy:  "policy2",
				Storage:       "storage2",
				Namespaces:    ptr.Of([]string{"ns2"}),
				IntervalCron:  "* * * * * *",
			},
		},
		AerospikeClusters: map[string]*AerospikeCluster{
			"cluster1": NewLocalAerospikeCluster(),
			"cluster2": NewLocalAerospikeCluster(),
		},
		BackupPolicies: map[string]*BackupPolicy{
			"policy1": {},
			"policy2": {},
		},
		Storage: map[string]*Storage{
			"storage1": {LocalStorage: &LocalStorage{Path: "/"}},
			"storage2": {LocalStorage: &LocalStorage{Path: "/"}},
		},
	}
}

// NewLocalAerospikeCluster returns a new AerospikeCluster to be used in tests.
func NewLocalAerospikeCluster() *AerospikeCluster {
	return &AerospikeCluster{
		SeedNodes:   []SeedNode{{HostName: "localhost", Port: 3000}},
		Credentials: &Credentials{User: "tester", Password: "psw"},
	}
}

func TestValidConfigValidation(t *testing.T) {
	config := validConfig()

	require.NoError(t, config.Validate(ValidationDefault))
}

func TestInvalidClusterReference(t *testing.T) {
	config := validConfig()
	routine := config.BackupRoutines["routine1"]
	routine.SourceCluster = "nonExistentCluster"

	_, err := config.ToModel()

	require.Error(t, err)
	require.ErrorIs(t, err, errNotFound)
	require.ErrorContains(t, err, "nonExistentCluster")
}

func TestInvalidBackupPolicyReference(t *testing.T) {
	config := validConfig()
	routine := config.BackupRoutines["routine1"]
	routine.BackupPolicy = "nonExistentPolicy"

	_, err := config.ToModel()
	require.Error(t, err)
	require.ErrorIs(t, err, errNotFound)
	require.ErrorContains(t, err, "nonExistentPolicy")
}

func TestInvalidStorageReference(t *testing.T) {
	config := validConfig()
	routine := config.BackupRoutines["routine1"]
	routine.Storage = "nonExistentStorage"

	_, err := config.ToModel()
	require.Error(t, err)
	require.ErrorIs(t, err, errNotFound)
	require.ErrorContains(t, err, "nonExistentStorage")
}

func TestInvalidTlsFile(t *testing.T) {
	config := validConfig()

	cluster := config.AerospikeClusters["cluster1"]
	cluster.SeedNodes[0].TLSName = "tls name"
	cluster.TLS = &TLS{
		ClientTLS: ClientTLS{
			Name:     "tls name",
			Keyfile:  "path to key file",
			Certfile: "path to cert file",
			CAFile:   "path to ca file",
		},
	}

	err := config.Validate(ValidationDefault)
	require.Error(t, err)

	require.NoError(t, config.Validate(ValidationSkipTLSFiles))
	_, err = config.ToModel()
	require.NoError(t, err)
}

func TestNewConfigFromReader(t *testing.T) {
	t.Parallel()

	yamlConfig := `
service:
`
	cfg, err := NewConfigFromReader(strings.NewReader(yamlConfig), decoder.YAML)
	require.NoError(t, err)
	require.NotNil(t, cfg)
}

func TestNewConfigFromReader_InvalidYAML(t *testing.T) {
	t.Parallel()

	_, err := NewConfigFromReader(strings.NewReader("service: [1,2"), decoder.YAML)
	require.Error(t, err)
}

func TestConfig_ToModel_ResolvesScheduleTimezone(t *testing.T) {
	t.Parallel()

	ny, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	t.Run("defaults to UTC", func(t *testing.T) {
		t.Parallel()

		modelConfig, err := validConfig().ToModel()
		require.NoError(t, err)
		assert.Equal(t, time.UTC, modelConfig.Routines()["routine1"].Timezone)
	})

	t.Run("inherits service timezone", func(t *testing.T) {
		t.Parallel()

		config := validConfig()
		config.ServiceConfig.Backup = &BackupCommonConfig{ScheduleTimezone: "America/New_York"}

		modelConfig, err := config.ToModel()
		require.NoError(t, err)
		assert.Equal(t, ny.String(), modelConfig.Routines()["routine1"].Timezone.String())
		assert.Equal(t, ny.String(), modelConfig.Routines()["routine2"].Timezone.String())
	})

	t.Run("whitespace routine timezone inherits service timezone", func(t *testing.T) {
		t.Parallel()

		config := validConfig()
		config.ServiceConfig.Backup = &BackupCommonConfig{ScheduleTimezone: "America/New_York"}
		config.BackupRoutines["routine1"].ScheduleTimezone = " \t "

		modelConfig, err := config.ToModel()
		require.NoError(t, err)
		routine := modelConfig.Routines()["routine1"]
		assert.Equal(t, ny.String(), routine.Timezone.String())
		assert.Empty(t, routine.ConfiguredTimezone)
	})

	t.Run("routine override wins", func(t *testing.T) {
		t.Parallel()

		config := validConfig()
		config.ServiceConfig.Backup = &BackupCommonConfig{ScheduleTimezone: "America/New_York"}
		config.BackupRoutines["routine1"].ScheduleTimezone = "UTC"

		modelConfig, err := config.ToModel()
		require.NoError(t, err)
		assert.Equal(t, time.UTC, modelConfig.Routines()["routine1"].Timezone)
		assert.Equal(t, ny.String(), modelConfig.Routines()["routine2"].Timezone.String())
	})
}

// TestConfig_ScheduleTimezoneRoundTrip asserts that Config -> model -> Config preserves
// both an explicit configured value and the resolved location, so a read-modify-write cycle
// neither drops an explicit value nor bakes in an inherited one.
func TestConfig_ScheduleTimezoneRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		serviceTimezone  string
		routineTimezone  string
		expectedResolved string
	}{
		{
			name:             "routine inherits service default",
			serviceTimezone:  "America/New_York",
			routineTimezone:  "",
			expectedResolved: "America/New_York",
		},
		{
			name:             "explicit value matching service default survives",
			serviceTimezone:  "America/New_York",
			routineTimezone:  "America/New_York",
			expectedResolved: "America/New_York",
		},
		{
			name:             "explicit UTC survives against non-UTC default",
			serviceTimezone:  "America/New_York",
			routineTimezone:  "UTC",
			expectedResolved: "UTC",
		},
		{
			name:             "explicit UTC survives against UTC default",
			serviceTimezone:  "",
			routineTimezone:  "UTC",
			expectedResolved: "UTC",
		},
		{
			name:             "everything omitted stays omitted",
			serviceTimezone:  "",
			routineTimezone:  "",
			expectedResolved: "UTC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := validConfig()
			if tt.serviceTimezone != "" {
				config.ServiceConfig.Backup = &BackupCommonConfig{ScheduleTimezone: tt.serviceTimezone}
			}
			config.BackupRoutines["routine1"].ScheduleTimezone = tt.routineTimezone

			modelConfig, err := config.ToModel()
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResolved, modelConfig.Routines()["routine1"].Timezone.String())

			roundTrip := NewConfigFromModel(modelConfig)
			require.NotNil(t, roundTrip)
			assert.Equal(t, tt.routineTimezone, roundTrip.BackupRoutines["routine1"].ScheduleTimezone,
				"configured routine value must survive the round trip")
			assert.Equal(t, config.ServiceConfig.Backup, roundTrip.ServiceConfig.Backup)

			// Reconverting must resolve to the same location: the omit/keep decision
			// has to be a fixpoint, not just behaviourally equivalent once.
			remodeled, err := roundTrip.ToModel()
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResolved, remodeled.Routines()["routine1"].Timezone.String())
		})
	}
}
