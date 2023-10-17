package service

import (
	"errors"
	"fmt"
	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/util"
)

// AddStorage
// adds a new BackupStorage to the configuration if a storage with the same name doesn't already exist.
func AddStorage(config *model.Config, newStorage *model.BackupStorage) error {
	_, existing := util.GetByName(config.BackupStorage, newStorage.Name)
	if existing != nil {
		return errors.New(fmt.Sprintf("Cluster %s not found", *newStorage.Name))
	}

	config.BackupStorage = append(config.BackupStorage, newStorage)
	return nil
}

// UpdateStorage
// updates an existing BackupStorage in the configuration
func UpdateStorage(config *model.Config, updatedStorage *model.BackupStorage) error {
	i, existing := util.GetByName(config.BackupStorage, updatedStorage.Name)
	if existing != nil {
		config.BackupStorage[i] = updatedStorage
		return nil
	}

	return errors.New(fmt.Sprintf("Storage %s not found", *updatedStorage.Name))
}

// DeleteStorage
// deletes a BackupStorage from the configuration if it is not used in any policy
func DeleteStorage(config *model.Config, storageToDeleteName *string) error {
	_, policy := util.Find(config.BackupPolicy, func(policy *model.BackupPolicy) bool {
		return *policy.Storage == *storageToDeleteName
	})

	if policy != nil {
		return errors.New(fmt.Sprintf("Cannot delete storage as it is used in a policy %s", *policy.Name))
	}

	i, existing := util.GetByName(config.BackupStorage, storageToDeleteName)
	if existing != nil {
		config.BackupStorage = append(config.BackupStorage[:i], config.BackupStorage[i+1:]...)
		return nil
	}
	return errors.New(fmt.Sprintf("Cluster %s not found", *storageToDeleteName))
}
