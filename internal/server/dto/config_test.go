package dto

import (
	"testing"

	"github.com/aws/smithy-go/ptr"
)

func validConfig() *Config {
	return &Config{
		ServiceConfig: NewBackupServiceConfigWithDefaultValues(),
		BackupRoutines: map[string]BackupRoutine{
			"routine1": {
				SourceCluster: "cluster1",
				BackupPolicy:  "policy1",
				Storage:       "storage1",
				Namespaces:    []string{"ns1"},
				IntervalCron:  "* * * * * *",
			},
			"routine2": {
				SourceCluster: "cluster2",
				BackupPolicy:  "policy2",
				Storage:       "storage2",
				Namespaces:    []string{"ns2"},
				IntervalCron:  "* * * * * *",
			},
		},
		AerospikeClusters: map[string]AerospikeCluster{
			"cluster1": NewLocalAerospikeCluster(),
			"cluster2": NewLocalAerospikeCluster(),
		},
		BackupPolicies: map[string]BackupPolicy{
			"policy1": {},
			"policy2": {},
		},
		Storage: map[string]Storage{
			"storage1": {Type: Local, Path: ptr.String("/")},
			"storage2": {Type: Local, Path: ptr.String("/")},
		},
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

	err := config.Validate()
	if err == nil {
		t.Fatalf("Expected validation error, but got none.")
	}
	expectedError := "backup routine 'routine1' validation error: Aerospike cluster 'nonExistentCluster' not found"
	if err.Error() != expectedError {
		t.Errorf("Expected error message '%s', but got '%s'", expectedError, err.Error())
	}
}

func TestInvalidBackupPolicyReference(t *testing.T) {
	config := validConfig()
	routine := config.BackupRoutines["routine1"]
	routine.BackupPolicy = "nonExistentPolicy"

	err := config.Validate()
	if err == nil {
		t.Fatalf("Expected validation error, but got none.")
	}
	expectedError := "backup routine 'routine1' validation error: backup policy 'nonExistentPolicy' not found"
	if err.Error() != expectedError {
		t.Errorf("Expected error message '%s', but got '%s'", expectedError, err.Error())
	}
}

func TestInvalidStorageReference(t *testing.T) {
	config := validConfig()
	routine := config.BackupRoutines["routine1"]
	routine.Storage = "nonExistentStorage"

	err := config.Validate()
	if err == nil {
		t.Fatalf("Expected validation error, but got none.")
	}
	expectedError := "backup routine 'routine1' validation error: storage 'nonExistentStorage' not found"
	if err.Error() != expectedError {
		t.Errorf("Expected error message '%s', but got '%s'", expectedError, err.Error())
	}
}
