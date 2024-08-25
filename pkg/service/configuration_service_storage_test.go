package service

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/pkg/model"
	"github.com/aws/smithy-go/ptr"
)

func TestStorage_Add(t *testing.T) {
	config := &model.Config{
		Storage: map[string]*model.Storage{},
	}

	name := "storage"
	newStorage := &model.Storage{
		Type: model.Local,
		Path: ptr.String("path"),
	}

	err := AddStorage(config, name, newStorage)
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}

	// Adding a storage with the same name as an existing one
	// should result in an error
	err = AddStorage(config, name, newStorage)
	if err == nil {
		t.Errorf("Expected an error, got nil")
	}
}

func TestStorage_AddFailure(t *testing.T) {
	config := &model.Config{
		Storage: map[string]*model.Storage{},
	}

	err := AddStorage(config, "storage", &model.Storage{})
	if err == nil {
		t.Errorf("Expected validation error")
	}
}

func TestStorage_Update(t *testing.T) {
	name := "name"
	config := &model.Config{
		Storage: map[string]*model.Storage{name: {}},
	}

	newStorage := &model.Storage{
		Path: ptr.String("path"),
		Type: model.Local,
	}

	err := UpdateStorage(config, name, newStorage)
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}

	if *config.Storage[name].Path != "path" {
		t.Errorf("Value in name is not updated")
	}

	// Updating a non-existent name should result in an error
	err = UpdateStorage(config, "newStorage", &model.Storage{})
	if err == nil {
		t.Errorf("Expected an error, got nil")
	}
}

func TestStorage_UpdateFailure(t *testing.T) {
	storage := "storage"
	config := &model.Config{
		Storage: map[string]*model.Storage{storage: {}},
	}

	err := UpdateStorage(config, storage, &model.Storage{})
	if err == nil {
		t.Errorf("Expected validation error")
	}
}

func TestStorage_Delete(t *testing.T) {
	storage := "storage"
	storage2 := "storage2"
	policy := "policy"
	routine := "routine"
	config := &model.Config{
		BackupPolicies: map[string]*model.BackupPolicy{policy: {}},
		Storage:        map[string]*model.Storage{storage: {}, storage2: {}},
		BackupRoutines: map[string]*model.BackupRoutine{routine: {Storage: storage}},
	}

	// Deleting a storage that is being used by a policy should result in an error
	err := DeleteStorage(config, storage)
	if err == nil {
		t.Errorf("Expected an error, got nil")
	}

	err = DeleteStorage(config, storage2)
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}
	if len(config.Storage) != 1 {
		t.Errorf("Expected config storage size to be 1")
	}

	// Deleting a non-existent storage should result in an error
	err = DeleteStorage(config, storage2)
	if err == nil {
		t.Errorf("Expected an error, got nil")
	}
}
