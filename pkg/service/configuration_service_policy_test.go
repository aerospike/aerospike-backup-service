package service

import (
	"github.com/aws/smithy-go/ptr"
	"testing"

	"github.com/aerospike/backup/pkg/model"
)

func TestAddPolicyOK(t *testing.T) {
	config := &model.Config{
		AerospikeClusters: []*model.AerospikeCluster{{Name: ptr.String("cluster")}},
		BackupStorage:     []*model.BackupStorage{{Name: ptr.String("storage")}},
	}

	pass := model.BackupPolicy{Storage: ptr.String("storage"), SourceCluster: ptr.String("cluster")}
	err := AddPolicy(config, &pass)
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}
}

func TestAddPolicyErrors(t *testing.T) {
	fails := []struct {
		name   string
		policy model.BackupPolicy
	}{
		{name: "empty", policy: model.BackupPolicy{}},
		{name: "no storage", policy: model.BackupPolicy{SourceCluster: ptr.String("cluster")}},
		{name: "no cluster", policy: model.BackupPolicy{Storage: ptr.String("storage")}},
		{name: "wrong storage", policy: model.BackupPolicy{Storage: ptr.String("_"), SourceCluster: ptr.String("cluster")}},
		{name: "wrong cluster", policy: model.BackupPolicy{Storage: ptr.String("storage"), SourceCluster: ptr.String("_")}},
		{name: "existing policy", policy: model.BackupPolicy{Name: ptr.String("policy")}},
	}

	config := &model.Config{
		BackupPolicy:      []*model.BackupPolicy{{Name: ptr.String("policy")}},
		AerospikeClusters: []*model.AerospikeCluster{{Name: ptr.String("cluster")}},
		BackupStorage:     []*model.BackupStorage{{Name: ptr.String("storage")}},
	}

	for _, testPolicy := range fails {
		err := AddPolicy(config, &testPolicy.policy)
		if err == nil {
			t.Errorf("Expected an error on %s", testPolicy.name)
		}
	}
}

func TestUpdatePolicy(t *testing.T) {
	config := &model.Config{
		BackupPolicy: []*model.BackupPolicy{{Name: ptr.String("policy1")}},
	}

	updatedPolicy := &model.BackupPolicy{
		Name: ptr.String("policy2"),
	}

	err := UpdatePolicy(config, updatedPolicy)
	expectedError := "Policy policy2 not found"
	if err.Error() != expectedError {
		t.Errorf("UpdatePolicy failed, expected error %s, got %v", expectedError, err)
	}

	updatedPolicy.Name = ptr.String("policy1")
	err = UpdatePolicy(config, updatedPolicy)
	if err != nil {
		t.Errorf("UpdatePolicy failed, expected nil error, got %v", err)
	}

	if *config.BackupPolicy[0].Name != *updatedPolicy.Name {
		t.Errorf("UpdatePolicy failed, expected policy name to be updated, got %v", *config.BackupPolicy[0].Name)
	}
}

func TestDeletePolicy(t *testing.T) {
	config := &model.Config{
		BackupPolicy: []*model.BackupPolicy{{Name: ptr.String("policy1")}},
	}

	err := DeletePolicy(config, ptr.String("policy2"))
	expectedError := "Policy policy2 not found"
	if err.Error() != expectedError {
		t.Errorf("DeletePolicy failed, expected error %s, got %v", expectedError, err)
	}

	err = DeletePolicy(config, ptr.String("policy1"))
	if err != nil {
		t.Errorf("DeletePolicy failed, expected nil error, got %v", err)
	}

	if len(config.BackupPolicy) != 0 {
		t.Errorf("DeletePolicy failed, expected policy to be deleted, got %d", len(config.BackupPolicy))
	}
}
