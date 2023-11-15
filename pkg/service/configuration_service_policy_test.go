package service

import (
	"testing"

	"github.com/aws/smithy-go/ptr"

	"github.com/aerospike/backup/pkg/model"
)

func TestAddPolicyOK(t *testing.T) {
	cluster := "cluster"
	storage := "storage"
	config := &model.Config{
		AerospikeClusters: map[string]*model.AerospikeCluster{cluster: {Name: &cluster}},
		Storages:          map[string]*model.Storage{storage: {Name: &storage}},
		BackupPolicies:    map[string]*model.BackupPolicy{},
	}

	pass := model.BackupPolicy{Name: ptr.String("newName"), Storage: &storage, SourceCluster: &cluster}
	err := AddPolicy(config, &pass)
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}
}

func TestAddPolicyErrors(t *testing.T) {
	policy := "policy"
	cluster := "cluster"
	storage := "storage"
	wrong := "-"
	fails := []struct {
		name   string
		policy model.BackupPolicy
	}{
		{name: "empty", policy: model.BackupPolicy{}},
		{name: "no storage", policy: model.BackupPolicy{SourceCluster: &cluster}},
		{name: "no cluster", policy: model.BackupPolicy{Storage: &storage}},
		{name: "wrong storage", policy: model.BackupPolicy{Storage: &wrong, SourceCluster: &cluster}},
		{name: "wrong cluster", policy: model.BackupPolicy{Storage: &storage, SourceCluster: &wrong}},
		{name: "existing policy", policy: model.BackupPolicy{Name: &policy}},
	}

	config := &model.Config{
		BackupPolicies:    map[string]*model.BackupPolicy{policy: {Name: &policy}},
		AerospikeClusters: map[string]*model.AerospikeCluster{policy: {Name: &cluster}},
		Storages:          map[string]*model.Storage{storage: {Name: &storage}},
	}

	for _, testPolicy := range fails {
		err := AddPolicy(config, &testPolicy.policy)
		if err == nil {
			t.Errorf("Expected an error on %s", testPolicy.name)
		}
	}
}

func TestUpdatePolicy(t *testing.T) {
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
		t.Errorf("UpdatePolicy failed, expected policy name to be updated, got %v", *config.BackupPolicies[name].Name)
	}
}

func TestDeletePolicy(t *testing.T) {
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
		t.Errorf("DeletePolicy failed, expected policy to be deleted, got %d", len(config.BackupPolicies))
	}
}
