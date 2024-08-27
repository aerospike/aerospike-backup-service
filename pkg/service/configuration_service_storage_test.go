package service

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/internal/server/dto"
	"github.com/aws/smithy-go/ptr"
)

func TestStorage_Add(t *testing.T) {
	config := &dto.Config{
		Storage: map[string]*dto.Storage{},
	}

	name := "storage"
	newStorage := &dto.Storage{
		Type: dto.Local,
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
	config := &dto.Config{
		Storage: map[string]*dto.Storage{},
	}

	err := AddStorage(config, "storage", &dto.Storage{})
	if err == nil {
		t.Errorf("Expected validation error")
	}
}

func TestStorage_Update(t *testing.T) {
	name := "name"
	config := &dto.Config{
		Storage: map[string]*dto.Storage{name: {}},
	}

	newStorage := &dto.Storage{
		Path: ptr.String("path"),
		Type: dto.Local,
	}

	err := UpdateStorage(config, name, newStorage)
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}

	if *config.Storage[name].Path != "path" {
		t.Errorf("Value in name is not updated")
	}

	// Updating a non-existent name should result in an error
	err = UpdateStorage(config, "newStorage", &dto.Storage{})
	if err == nil {
		t.Errorf("Expected an error, got nil")
	}
}

func TestStorage_UpdateFailure(t *testing.T) {
	storage := "storage"
	config := &dto.Config{
		Storage: map[string]*dto.Storage{storage: {}},
	}

	err := UpdateStorage(config, storage, &dto.Storage{})
	if err == nil {
		t.Errorf("Expected validation error")
	}
}

func TestStorage_Delete(t *testing.T) {
	storage := "storage"
	storage2 := "storage2"
	policy := "policy"
	routine := "routine"
	config := &dto.Config{
		BackupPolicies: map[string]*dto.BackupPolicy{policy: {}},
		Storage:        map[string]*dto.Storage{storage: {}, storage2: {}},
		BackupRoutines: map[string]*dto.BackupRoutine{routine: {Storage: storage}},
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
