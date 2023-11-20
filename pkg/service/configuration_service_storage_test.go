package service

import (
	"testing"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/util"
	"github.com/aws/smithy-go/ptr"
)

func TestStorage_Add(t *testing.T) {
	config := &model.Config{
		Storage: map[string]*model.Storage{},
	}

	newStorage := &model.Storage{
		Name: ptr.String("storage"),
		Type: util.Ptr(model.Local),
		Path: ptr.String("path"),
	}

	err := AddStorage(config, newStorage)
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}

	// Adding a storage with the same name as an existing one
	// should result in an error
	err = AddStorage(config, newStorage)
	if err == nil {
		t.Errorf("Expected an error, got nil")
	}
}

func TestStorage_AddFailure(t *testing.T) {
	config := &model.Config{
		Storage: map[string]*model.Storage{},
	}

	newStorage := &model.Storage{
		Name: ptr.String("storage"),
	}

	err := AddStorage(config, newStorage)
	if err == nil {
		t.Errorf("Expected validation error")
	}
}

func TestStorage_Update(t *testing.T) {
	storage := "storage"
	config := &model.Config{
		Storage: map[string]*model.Storage{storage: {Name: &storage}},
	}

	newStorage := &model.Storage{
		Name: ptr.String(storage),
		Path: ptr.String("path"),
		Type: util.Ptr(model.Local),
	}

	err := UpdateStorage(config, newStorage)
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}

	if *config.Storage[storage].Path != "path" {
		t.Errorf("Value in storage is not updated")
	}

	// Updating a non-existent storage should result in an error
	newStorage = &model.Storage{Name: ptr.String("newStorage")}
	err = UpdateStorage(config, newStorage)
	if err == nil {
		t.Errorf("Expected an error, got nil")
	}
}

func TestStorage_UpdateFailure(t *testing.T) {
	storage := "storage"
	config := &model.Config{
		Storage: map[string]*model.Storage{storage: {Name: &storage}},
	}

	newStorage := &model.Storage{
		Name: ptr.String(storage),
	}

	err := UpdateStorage(config, newStorage)
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
		BackupPolicies: map[string]*model.BackupPolicy{policy: {Name: &policy}},
		Storage:        map[string]*model.Storage{storage: {Name: &storage}, storage2: {Name: &storage2}},
		BackupRoutines: map[string]*model.BackupRoutine{routine: {Name: routine, Storage: storage}},
	}

	// Deleting a storage that is being used by a policy should result in an error
	err := DeleteStorage(config, &storage)
	if err == nil {
		t.Errorf("Expected an error, got nil")
	}

	err = DeleteStorage(config, &storage2)
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}
	if len(config.Storage) != 1 {
		t.Errorf("Expected config storage size to be 1")
	}

	// Deleting a non-existent storage should result in an error
	err = DeleteStorage(config, &storage2)
	if err == nil {
		t.Errorf("Expected an error, got nil")
	}
}
