package service

import (
	"testing"

	"github.com/aws/smithy-go/ptr"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/util"
)

func TestAddStorage(t *testing.T) {
	config := &model.Config{
		BackupStorage: []*model.BackupStorage{},
	}

	newStorage := &model.BackupStorage{
		Name: ptr.String("storage"),
		Type: util.Ptr(model.Local),
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
	config := &model.Config{
		BackupStorage: []*model.BackupStorage{{Name: ptr.String("storage")}},
	}

	newStorage := &model.BackupStorage{
		Name: ptr.String("storage"),
		Path: ptr.String("path"),
		Type: util.Ptr(model.Local),
	}

	err := UpdateStorage(config, newStorage)
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}

	if *config.BackupStorage[0].Path != "path" {
		t.Errorf("Value in storage is not updated")
	}

	// Updating a non-existent storage should result in an error
	newStorage = &model.BackupStorage{Name: ptr.String("newStorage")}
	err = UpdateStorage(config, newStorage)
	if err == nil {
		t.Errorf("Expected an error, got nil")
	}
}

func TestDeleteStorage(t *testing.T) {
	config := &model.Config{
		BackupPolicy:  []*model.BackupPolicy{{Name: ptr.String("policy"), Storage: ptr.String("storage")}},
		BackupStorage: []*model.BackupStorage{{Name: ptr.String("storage")}, {Name: ptr.String("storage2")}},
	}

	// Deleting a storage that is being used by a policy should result in an error
	err := DeleteStorage(config, ptr.String("storage"))
	if err == nil {
		t.Errorf("Expected an error, got nil")
	}

	err = DeleteStorage(config, ptr.String("storage2"))
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}
	if len(config.BackupStorage) != 1 {
		t.Errorf("Expected size = 1")
	}

	// Deleting a non-existent storage should result in an error
	err = DeleteStorage(config, ptr.String("storage2"))
	if err == nil {
		t.Errorf("Expected an error, got nil")
	}
}
