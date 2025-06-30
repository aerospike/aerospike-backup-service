package dto

import (
	"errors"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
)

func validConfig() *Config {
	return &Config{
		ServiceConfig: BackupServiceConfig{},
		BackupRoutines: map[string]*BackupRoutine{
			"routine1": {
				SourceCluster: "cluster1",
				BackupPolicy:  "policy1",
				Storage:       "storage1",
				Namespaces:    util.Ptr([]string{"ns1"}),
				IntervalCron:  "* * * * * *",
			},
			"routine2": {
				SourceCluster: "cluster2",
				BackupPolicy:  "policy2",
				Storage:       "storage2",
				Namespaces:    util.Ptr([]string{"ns2"}),
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
			"storage1": {LocalStorage: &LocalStorage{"/"}},
			"storage2": {LocalStorage: &LocalStorage{"/"}},
		},
	}
}

// NewLocalAerospikeCluster returns a new AerospikeCluster to be used in tests.
func NewLocalAerospikeCluster() *AerospikeCluster {
	return &AerospikeCluster{
		SeedNodes:   []SeedNode{{HostName: "localhost", Port: 3000}},
		Credentials: &Credentials{User: util.Ptr("tester"), Password: util.Ptr("psw")},
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

	_, err := config.ToModel(&aerospike.NoopNamespaceValidator{})

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

	_, err := config.ToModel(&aerospike.NoopNamespaceValidator{})
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

	_, err := config.ToModel(&aerospike.NoopNamespaceValidator{})
	if err == nil {
		t.Fatalf("Expected validation error, but got none.")
	}
	expectedError := errValidationNotFound("routine1", "nonExistentStorage")
	if errors.Is(err, expectedError) {
		t.Errorf("Expected error message '%s', but got '%s'", expectedError, err.Error())
	}
}
