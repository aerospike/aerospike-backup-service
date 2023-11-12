//nolint:dupl
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
		return fmt.Errorf("storage %s already exists", *newStorage.Name)
	}
	if err := validate(newStorage); err != nil {
		return err
	}

	config.BackupStorage = append(config.BackupStorage, newStorage)
	return nil
}

// UpdateStorage
// updates an existing BackupStorage in the configuration.
func UpdateStorage(config *model.Config, updatedStorage *model.BackupStorage) error {
	i, existing := util.GetByName(config.BackupStorage, updatedStorage.Name)
	if existing == nil {
		return fmt.Errorf("storage %s not found", *updatedStorage.Name)
	}
	if err := validate(updatedStorage); err != nil {
		return err
	}

	config.BackupStorage[i] = updatedStorage
	return nil
}

// DeleteStorage
// deletes a BackupStorage from the configuration if it is not used in any policy.
func DeleteStorage(config *model.Config, storageToDeleteName *string) error {
	_, policy := util.Find(config.BackupPolicy, func(policy *model.BackupPolicy) bool {
		return *policy.Storage == *storageToDeleteName
	})

	if policy != nil {
		return fmt.Errorf("cannot delete storage as it is used in a policy %s", *policy.Name)
	}

	i, existing := util.GetByName(config.BackupStorage, storageToDeleteName)
	if existing != nil {
		config.BackupStorage = append(config.BackupStorage[:i], config.BackupStorage[i+1:]...)
		return nil
	}
	return fmt.Errorf("cluster %s not found", *storageToDeleteName)
}

func validate(b *model.BackupStorage) error {
	if b.Name == nil || *b.Name == "" {
		return errors.New("storage name is required")
	}
	if b.Type == nil {
		return errors.New("storage type is required")
	}
	if b.Path == nil {
		return errors.New("storage path is required")
	}
	return nil
}
