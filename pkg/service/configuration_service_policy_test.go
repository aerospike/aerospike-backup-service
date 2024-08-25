package service

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/pkg/model"
	"github.com/aws/smithy-go/ptr"
)

func TestPolicy_AddOK(t *testing.T) {
	cluster := "cluster"
	storage := "storage"
	config := &model.Config{
		AerospikeClusters: map[string]*model.AerospikeCluster{cluster: {}},
		Storage:           map[string]*model.Storage{storage: {}},
		BackupPolicies:    map[string]*model.BackupPolicy{},
	}

	err := AddPolicy(config, "newName", &model.BackupPolicy{})
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}
}

func TestPolicy_AddErrors(t *testing.T) {
	policy := "policy"
	cluster := "cluster"
	storage := "storage"
	fails := []struct {
		name   string
		policy model.BackupPolicy
	}{
		{name: "existing policy", policy: model.BackupPolicy{}},
	}

	config := &model.Config{
		BackupPolicies:    map[string]*model.BackupPolicy{policy: {}},
		AerospikeClusters: map[string]*model.AerospikeCluster{cluster: {}},
		Storage:           map[string]*model.Storage{storage: {}},
	}

	for i := range fails {
		err := AddPolicy(config, policy, &fails[i].policy)
		if err == nil {
			t.Errorf("Expected an error on %s", fails[i].name)
		}
	}
}

func TestPolicy_Update(t *testing.T) {
	name := "policy1"
	config := &model.Config{
		BackupPolicies: map[string]*model.BackupPolicy{name: {}},
	}

	err := UpdatePolicy(config, "policy2", &model.BackupPolicy{})
	if err == nil {
		t.Errorf("UpdatePolicy failed, expected policy not found error")
	}

	err = UpdatePolicy(config, name, &model.BackupPolicy{
		MaxRetries: ptr.Int32(10),
	})
	if err != nil {
		t.Errorf("UpdatePolicy failed, expected nil error, got %v", err)
	}

	if *config.BackupPolicies[name].MaxRetries != 10 {
		t.Errorf("UpdatePolicy failed, expected MaxRetries to be updated, got %v",
			*config.BackupPolicies[name])
	}
}

func TestPolicy_Delete(t *testing.T) {
	name := "policy1"
	config := &model.Config{
		BackupPolicies: map[string]*model.BackupPolicy{name: {}},
	}

	err := DeletePolicy(config, "policy2")
	if err == nil {
		t.Errorf("DeletePolicy failed, expected nil error, got %v", err)
	}

	err = DeletePolicy(config, "policy1")
	if err != nil {
		t.Errorf("DeletePolicy failed, expected nil error, got %v", err)
	}

	if len(config.BackupPolicies) != 0 {
		t.Errorf("DeletePolicy failed, expected policy to be deleted, got %d",
			len(config.BackupPolicies))
	}
}
