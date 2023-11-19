package service

import (
	"testing"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aws/smithy-go/ptr"
)

func TestPolicy_AddOK(t *testing.T) {
	cluster := "cluster"
	storage := "storage"
	config := &model.Config{
		AerospikeClusters: map[string]*model.AerospikeCluster{cluster: {Name: &cluster}},
		Storage:           map[string]*model.Storage{storage: {Name: &storage}},
		BackupPolicies:    map[string]*model.BackupPolicy{},
	}

	pass := model.BackupPolicy{Name: ptr.String("newName")}
	err := AddPolicy(config, &pass)
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
		{name: "existing policy", policy: model.BackupPolicy{Name: &policy}},
	}

	config := &model.Config{
		BackupPolicies:    map[string]*model.BackupPolicy{policy: {Name: &policy}},
		AerospikeClusters: map[string]*model.AerospikeCluster{cluster: {Name: &cluster}},
		Storage:           map[string]*model.Storage{storage: {Name: &storage}},
	}

	for _, testPolicy := range fails {
		err := AddPolicy(config, &testPolicy.policy)
		if err == nil {
			t.Errorf("Expected an error on %s", testPolicy.name)
		}
	}
}

func TestPolicy_Update(t *testing.T) {
	name := "policy1"
	config := &model.Config{
		BackupPolicies: map[string]*model.BackupPolicy{name: {Name: &name}},
	}

	updatedPolicy := &model.BackupPolicy{
		Name: ptr.String("policy2"),
	}

	err := UpdatePolicy(config, updatedPolicy)
	if err == nil {
		t.Errorf("UpdatePolicy failed, expected policy not found error")
	}

	updatedPolicy.Name = &name
	err = UpdatePolicy(config, updatedPolicy)
	if err != nil {
		t.Errorf("UpdatePolicy failed, expected nil error, got %v", err)
	}

	if *config.BackupPolicies[name].Name != *updatedPolicy.Name {
		t.Errorf("UpdatePolicy failed, expected policy name to be updated, got %v",
			*config.BackupPolicies[name].Name)
	}
}

func TestPolicy_Delete(t *testing.T) {
	name := "policy1"
	config := &model.Config{
		BackupPolicies: map[string]*model.BackupPolicy{name: {Name: &name}},
	}

	err := DeletePolicy(config, ptr.String("policy2"))
	if err == nil {
		t.Errorf("DeletePolicy failed, expected nil error, got %v", err)
	}

	err = DeletePolicy(config, ptr.String("policy1"))
	if err != nil {
		t.Errorf("DeletePolicy failed, expected nil error, got %v", err)
	}

	if len(config.BackupPolicies) != 0 {
		t.Errorf("DeletePolicy failed, expected policy to be deleted, got %d",
			len(config.BackupPolicies))
	}
}
