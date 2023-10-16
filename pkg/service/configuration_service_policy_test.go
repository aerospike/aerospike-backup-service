package service

import (
	"github.com/aws/smithy-go/ptr"
	"testing"

	"github.com/aerospike/backup/pkg/model"
)

func TestAddPolicy(t *testing.T) {
	config := &model.Config{
		BackupPolicy: []*model.BackupPolicy{{Name: ptr.String("policy1")}},
	}

	newPolicy := &model.BackupPolicy{
		Name: ptr.String("policy2"),
	}

	err := AddPolicy(config, newPolicy)
	if err != nil {
		t.Errorf("AddPolicy failed, expected nil error, got %v", err)
	}

	err = AddPolicy(config, newPolicy)
	if err == nil {
		t.Errorf("Expected an error on adding existing policy")
	}
}

func TestUpdatePolicy(t *testing.T) {
	config := &model.Config{
		BackupPolicy: []*model.BackupPolicy{{Name: ptr.String("policy1")}},
	}

	updatedPolicy := model.BackupPolicy{
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

	policyToDeleteName := "policy2"

	err := DeletePolicy(config, policyToDeleteName)
	expectedError := "Policy policy2 not found"
	if err.Error() != expectedError {
		t.Errorf("DeletePolicy failed, expected error %s, got %v", expectedError, err)
	}

	err = DeletePolicy(config, "policy1")
	if err != nil {
		t.Errorf("DeletePolicy failed, expected nil error, got %v", err)
	}

	if len(config.BackupPolicy) != 0 {
		t.Errorf("DeletePolicy failed, expected policy to be deleted, got %d", len(config.BackupPolicy))
	}
}
