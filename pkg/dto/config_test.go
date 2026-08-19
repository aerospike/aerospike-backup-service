package dto

import (
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
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

	_, err := config.ToModel(ValidationDefault)

	require.Error(t, err)
	require.ErrorIs(t, err, errNotFound)
	require.ErrorContains(t, err, "nonExistentCluster")
}

func TestInvalidBackupPolicyReference(t *testing.T) {
	config := validConfig()
	routine := config.BackupRoutines["routine1"]
	routine.BackupPolicy = "nonExistentPolicy"

	_, err := config.ToModel(ValidationDefault)
	require.Error(t, err)
	require.ErrorIs(t, err, errNotFound)
	require.ErrorContains(t, err, "nonExistentPolicy")
}

func TestInvalidStorageReference(t *testing.T) {
	config := validConfig()
	routine := config.BackupRoutines["routine1"]
	routine.Storage = "nonExistentStorage"

	_, err := config.ToModel(ValidationDefault)
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

	_, err := config.ToModel(ValidationDefault)
	require.Error(t, err)

	_, err = config.ToModel(ValidationSkipTLSFiles)
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
