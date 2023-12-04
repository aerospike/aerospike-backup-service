package model

import (
	"github.com/aws/smithy-go/ptr"
	"testing"
)

func validConfig() *Config {
	return &Config{
		BackupRoutines: map[string]*BackupRoutine{
			"routine1": {
				SourceCluster: "cluster1",
				BackupPolicy:  "policy1",
				Storage:       "storage1", Namespace: "ns1",
				IntervalMillis: ptr.Int64(1)},
			"routine2": {
				SourceCluster:      "cluster2",
				BackupPolicy:       "policy2",
				Storage:            "storage2",
				Namespace:          "ns2",
				IncrIntervalMillis: ptr.Int64(1)},
		},
		AerospikeClusters: map[string]*AerospikeCluster{
			"cluster1": {Host: ptr.String("localhost"), Port: ptr.Int32(3000)},
			"cluster2": {Host: ptr.String("localhost"), Port: ptr.Int32(3000)},
		},
		BackupPolicies: map[string]*BackupPolicy{
			"policy1": {},
			"policy2": {},
		},
		Storage: map[string]*Storage{
			"storage1": {Path: ptr.String("/")},
			"storage2": {Path: ptr.String("/")},
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
	config.BackupRoutines["routine1"].SourceCluster = "nonExistentCluster"

	err := config.Validate()
	if err == nil {
		t.Error("Expected validation error, but got none.")
	}
	expectedError := "BackupRoutine 'routine1' references a non-existent AerospikeCluster 'nonExistentCluster'"
	if err.Error() != expectedError {
		t.Errorf("Expected error message '%s', but got '%s'", expectedError, err.Error())
	}
}

func TestInvalidBackupPolicyReference(t *testing.T) {
	config := validConfig()
	config.BackupRoutines["routine1"].BackupPolicy = "nonExistentPolicy"

	err := config.Validate()
	if err == nil {
		t.Error("Expected validation error, but got none.")
	}
	expectedError := "BackupRoutine 'routine1' references a non-existent BackupPolicy 'nonExistentPolicy'"
	if err.Error() != expectedError {
		t.Errorf("Expected error message '%s', but got '%s'", expectedError, err.Error())
	}
}

func TestInvalidStorageReference(t *testing.T) {
	config := validConfig()
	config.BackupRoutines["routine1"].Storage = "nonExistentStorage"

	err := config.Validate()
	if err == nil {
		t.Error("Expected validation error, but got none.")
	}
	expectedError := "BackupRoutine 'routine1' references a non-existent Storage 'nonExistentStorage'"
	if err.Error() != expectedError {
		t.Errorf("Expected error message '%s', but got '%s'", expectedError, err.Error())
	}
}
