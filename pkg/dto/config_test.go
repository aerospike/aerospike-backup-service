package dto

import (
	"errors"
	"testing"

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

	if err := config.Validate(); err != nil {
		t.Errorf("Expected no validation error, but got: %v", err)
	}
}

func TestInvalidClusterReference(t *testing.T) {
	config := validConfig()
	routine := config.BackupRoutines["routine1"]
	routine.SourceCluster = "nonExistentCluster"

	_, err := config.ToModel()

	if err == nil {
		t.Fatalf("Expected validation error, but got none.")
	}
	expectedError := errValidationNotFound("routine1", "nonExistentCluster")
	if errors.Is(err, expectedError) {
		t.Errorf("Expected error message '%s', but got '%s'", expectedError, err.Error())
	}
}

func TestInvalidBackupPolicyReference(t *testing.T) {
	config := validConfig()
	routine := config.BackupRoutines["routine1"]
	routine.BackupPolicy = "nonExistentPolicy"

	_, err := config.ToModel()
	if err == nil {
		t.Fatalf("Expected validation error, but got none.")
	}
	expectedError := errValidationNotFound("routine1", "nonExistentPolicy")
	if errors.Is(err, expectedError) {
		t.Errorf("Expected error message '%s', but got '%s'", expectedError, err.Error())
	}
}

func TestInvalidStorageReference(t *testing.T) {
	config := validConfig()
	routine := config.BackupRoutines["routine1"]
	routine.Storage = "nonExistentStorage"

	_, err := config.ToModel()
	if err == nil {
		t.Fatalf("Expected validation error, but got none.")
	}
	expectedError := errValidationNotFound("routine1", "nonExistentStorage")
	if errors.Is(err, expectedError) {
		t.Errorf("Expected error message '%s', but got '%s'", expectedError, err.Error())
	}
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

	_, err := config.ToModel()
	require.Error(t, err)

	_, err = config.ToModel(ValidationSkipTLSFiles)
	require.NoError(t, err)
}
