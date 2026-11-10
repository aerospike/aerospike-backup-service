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

func validConfig() *Config {
	return &Config{
		ServiceConfig: ServiceConfig{},
		BackupRoutines: map[string]*BackupRoutine{
			"routine1": {
				SourceCluster: "cluster1",
				BackupPolicy:  "policy1",
				Storage:       "storage1",
				Namespaces:    new([]NamespaceName{"ns1"}),
				IntervalCron:  "* * * * * *",
			},
			"routine2": {
				SourceCluster: "cluster2",
				BackupPolicy:  "policy2",
				Storage:       "storage2",
				Namespaces:    new([]NamespaceName{"ns2"}),
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

	require.NoError(t, config.Validate())
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

func TestConfigValidate_ServerMode(t *testing.T) {
	s3 := &Storage{S3Storage: &S3Storage{Bucket: "bucket", S3Region: "eu-central-1", Path: "backups"}}
	local := &Storage{LocalStorage: &LocalStorage{Path: "/"}}

	none := func(*Config) {}

	tests := []struct {
		name    string
		mode    BackupMode
		storage *Storage
		change  func(*Config)
		wantErr string
	}{
		{name: "server mode with S3", mode: BackupModeServer, storage: s3, change: none},
		{name: "scan mode with local", mode: BackupModeScan, storage: local, change: none},
		{
			name:    "scan mode with options the server lacks",
			mode:    BackupModeScan,
			storage: local,
			change: func(c *Config) {
				c.BackupRoutines["routine1"].NodeList = []string{"node"}
				c.BackupPolicies["policy1"].Bandwidth = new(int64(1))
			},
		},
		{
			name:    "server mode with local",
			mode:    BackupModeServer,
			storage: local,
			change:  none,
			wantErr: `backup routine 'routine1' validation error: invalid value validation error: storage "storage1" ` +
				`is not S3 storage, which backup-mode "server" requires`,
		},
		{
			name:    "server mode with a rack list",
			mode:    BackupModeServer,
			storage: s3,
			change:  func(c *Config) { c.BackupRoutines["routine1"].RackList = []int{1} },
			wantErr: `invalid value validation error: backup routine 'routine1' sets rack-list, ` +
				`which backup-mode "server" does not support`,
		},
		{
			name:    "server mode with a partition list",
			mode:    BackupModeServer,
			storage: s3,
			change:  func(c *Config) { c.BackupRoutines["routine1"].PartitionList = "0-100" },
			wantErr: `invalid value validation error: backup routine 'routine1' sets partition-list, ` +
				`which backup-mode "server" does not support`,
		},
		{
			name:    "server mode with a node list",
			mode:    BackupModeServer,
			storage: s3,
			change:  func(c *Config) { c.BackupRoutines["routine1"].NodeList = []string{"node"} },
			wantErr: `invalid value validation error: backup routine 'routine1' sets node-list, ` +
				`which backup-mode "server" does not support`,
		},
		{
			name:    "server mode with policy options it lacks",
			mode:    BackupModeServer,
			storage: s3,
			change: func(c *Config) {
				policy := c.BackupPolicies["policy1"]
				policy.Bandwidth = new(int64(1))
				policy.EncryptionPolicy = &EncryptionPolicy{Mode: EncryptionModeAES256, KeyFile: "key"}
				policy.CompressionPolicy = &CompressionPolicy{Mode: CompressionModeZSTD, Level: new(int32(1))}
				policy.NoRecords = new(true)
				policy.Compact = new(true)
				policy.FileLimit = new(1)
				policy.MaxConcurrentNodes = new(1)
				policy.RetryPolicy = &RetryPolicy{}
				policy.RetentionPolicy = &RetentionPolicy{FullBackups: new(1)}
				policy.SocketTimeout = new(int64(1))
				policy.TotalTimeout = new(int64(1))
			},
			wantErr: `invalid value validation error: policy 'policy1' sets bandwidth, encryption, compression, ` +
				`no-records, compact, file-limit, max-concurrent-nodes, retry-policy, retention, socket-timeout, ` +
				`total-timeout, which backup-mode "server" does not support`,
		},
		{
			name:    "server mode with an empty policy",
			mode:    BackupModeServer,
			storage: s3,
			change:  func(c *Config) { c.BackupPolicies["empty"] = nil },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := validConfig()
			delete(config.BackupRoutines, "routine2")
			delete(config.BackupPolicies, "policy2")
			config.Storage["storage1"] = tt.storage
			config.ServiceConfig.Backup = &BackupCommonConfig{BackupMode: tt.mode}
			tt.change(config)

			err := config.Validate()

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, errInvalidValue)
			require.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestTLSFilesAreNotReadDuringValidation(t *testing.T) {
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

	err := config.Validate()
	require.NoError(t, err)
	_, err = config.ToModel()
	require.NoError(t, err)
}

func TestNewConfigFromReader(t *testing.T) {
	t.Parallel()

	yamlConfig := `
service:
`
	cfg, err := NewFromReader[Config](strings.NewReader(yamlConfig), decoder.YAML)
	require.NoError(t, err)
	require.NotNil(t, cfg)
}

func TestNewConfigFromReader_InvalidYAML(t *testing.T) {
	t.Parallel()

	_, err := NewFromReader[Config](strings.NewReader("service: [1,2"), decoder.YAML)
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
		routine1 := modelConfig.Routines()["routine1"]
		assert.Equal(t, model.DefaultScheduleTimezone, routine1.Timezone.ResolvedLocation())
		assert.False(t, routine1.Timezone.IsExplicit())
	})

	t.Run("inherits service timezone", func(t *testing.T) {
		t.Parallel()

		config := validConfig()
		config.ServiceConfig.Backup = &BackupCommonConfig{ScheduleTimezone: "America/New_York"}

		modelConfig, err := config.ToModel()
		require.NoError(t, err)
		routine1 := modelConfig.Routines()["routine1"]
		routine2 := modelConfig.Routines()["routine2"]
		assert.Equal(t, ny.String(), routine1.Timezone.ResolvedLocation().String())
		assert.False(t, routine1.Timezone.IsExplicit())
		assert.Equal(t, ny.String(), routine2.Timezone.ResolvedLocation().String())
		assert.False(t, routine2.Timezone.IsExplicit())
	})

	t.Run("whitespace routine timezone inherits service timezone", func(t *testing.T) {
		t.Parallel()

		config := validConfig()
		config.ServiceConfig.Backup = &BackupCommonConfig{ScheduleTimezone: "America/New_York"}
		config.BackupRoutines["routine1"].ScheduleTimezone = " \t "

		modelConfig, err := config.ToModel()
		require.NoError(t, err)
		routine := modelConfig.Routines()["routine1"]
		assert.Equal(t, ny.String(), routine.Timezone.ResolvedLocation().String())
		assert.Equal(t, " \t ", routine.Timezone.Configured)
		assert.False(t, routine.Timezone.IsExplicit())
	})

	t.Run("routine override wins", func(t *testing.T) {
		t.Parallel()

		config := validConfig()
		config.ServiceConfig.Backup = &BackupCommonConfig{ScheduleTimezone: "America/New_York"}
		config.BackupRoutines["routine1"].ScheduleTimezone = "UTC"

		modelConfig, err := config.ToModel()
		require.NoError(t, err)
		routine1 := modelConfig.Routines()["routine1"]
		routine2 := modelConfig.Routines()["routine2"]
		assert.Equal(t, model.DefaultScheduleTimezone, routine1.Timezone.ResolvedLocation())
		assert.True(t, routine1.Timezone.IsExplicit())
		assert.Equal(t, ny.String(), routine2.Timezone.ResolvedLocation().String())
		assert.False(t, routine2.Timezone.IsExplicit())
	})
}

// TestConfig_ScheduleTimezoneRoundTrip asserts that Config -> model -> Config preserves
// both an explicit configured value and the resolved location, so a read-modify-write cycle
// neither drops an explicit value nor bakes in an inherited one.
func TestConfig_ScheduleTimezoneRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		serviceTimezone  ScheduleTimezone
		routineTimezone  ScheduleTimezone
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
			name:             "configured keyword casing survives",
			serviceTimezone:  "local",
			routineTimezone:  "utc",
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
			assert.Equal(t, tt.expectedResolved, modelConfig.Routines()["routine1"].Timezone.ResolvedLocation().String())

			roundTrip := NewConfigFromModel(modelConfig)
			require.NotNil(t, roundTrip)
			assert.Equal(t, tt.routineTimezone, roundTrip.BackupRoutines["routine1"].ScheduleTimezone,
				"configured routine value must survive the round trip")
			assert.Equal(t, config.ServiceConfig.Backup, roundTrip.ServiceConfig.Backup)

			// Reconverting must resolve to the same location: the omit/keep decision
			// has to be a fixpoint, not just behaviourally equivalent once.
			remodeled, err := roundTrip.ToModel()
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResolved, remodeled.Routines()["routine1"].Timezone.ResolvedLocation().String())
		})
	}
}
