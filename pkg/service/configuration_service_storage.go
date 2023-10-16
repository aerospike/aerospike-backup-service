package service

import (
	"errors"
	"fmt"
	"github.com/aerospike/backup/pkg/model"
)

// AddStorage
// adds a new BackupStorage to the configuration if a storage with the same name doesn't already exist.
func AddStorage(config *model.Config, newStorage *model.BackupStorage) error {
	for _, storage := range config.BackupStorage {
		if *storage.Name == *newStorage.Name {
			errorMessage := fmt.Sprintf("Aerospike cluster with the same name %s already exists", *newStorage.Name)
			return errors.New(errorMessage)
		}
	}

	config.BackupStorage = append(config.BackupStorage, newStorage)
	return nil
}

// UpdateStorage
// updates an existing BackupStorage in the configuration
func UpdateStorage(config *model.Config, updatedStorage model.BackupStorage) error {
	for i, storage := range config.BackupStorage {
		if *storage.Name == *updatedStorage.Name {
			config.BackupStorage[i] = &updatedStorage
			return nil
		}
	}
	return errors.New(fmt.Sprintf("Storage %s not found", *updatedStorage.Name))
}

// DeleteStorage
// deletes a BackupStorage from the configuration if it is not used in any policy
func DeleteStorage(config *model.Config, storageToDeleteName string) error {
	for _, policy := range config.BackupPolicy {
		if *policy.Storage == storageToDeleteName {
			return errors.New(fmt.Sprintf("Cannot delete storage as it is used in a policy %s", *policy.Name))
		}
	}

	for i, storage := range config.BackupStorage {
		if *storage.Name == storageToDeleteName {
			config.BackupStorage = append(config.BackupStorage[:i], config.BackupStorage[i+1:]...)
			return nil
		}
	}
	return errors.New(fmt.Sprintf("Cluster %s not found", storageToDeleteName))
}
