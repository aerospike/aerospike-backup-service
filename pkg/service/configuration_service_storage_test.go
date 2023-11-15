package service

import (
	"testing"

	"github.com/aws/smithy-go/ptr"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/util"
)

func TestAddStorage(t *testing.T) {
	config := &model.Config{
		Storages: map[string]*model.Storage{},
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

func TestUpdateStorage(t *testing.T) {
	storage := "storage"
	config := &model.Config{
		Storages: map[string]*model.Storage{storage: {Name: &storage}},
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

	if *config.Storages[storage].Path != "path" {
		t.Errorf("Value in storage is not updated")
	}

	// Updating a non-existent storage should result in an error
	newStorage = &model.Storage{Name: ptr.String("newStorage")}
	err = UpdateStorage(config, newStorage)
	if err == nil {
		t.Errorf("Expected an error, got nil")
	}
}

func TestDeleteStorage(t *testing.T) {
	policy := "policy"
	storage := "storage"
	storage2 := "storage2"
	config := &model.Config{
		BackupPolicies: map[string]*model.BackupPolicy{policy: {Name: &policy, Storage: &storage}},
		Storages:       map[string]*model.Storage{storage: {Name: &storage}, storage2: {Name: &storage2}},
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
	if len(config.Storages) != 1 {
		t.Errorf("Expected size = 1")
	}

	// Deleting a non-existent storage should result in an error
	err = DeleteStorage(config, &storage2)
	if err == nil {
		t.Errorf("Expected an error, got nil")
	}
}
