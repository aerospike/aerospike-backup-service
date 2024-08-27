//nolint:dupl
package service

import (
	"fmt"

	"github.com/aerospike/aerospike-backup-service/internal/server/dto"
	"github.com/aerospike/aerospike-backup-service/pkg/util"
)

// AddStorage adds a new Storage to the configuration if a storage with the same name
// doesn't already exist.
func AddStorage(config *dto.Config, name string, newStorage *dto.Storage) error {
	_, found := config.Storage[name]
	if found {
		return fmt.Errorf("storage %s already exists", name)
	}
	if err := newStorage.Validate(); err != nil {
		return err
	}

	config.Storage[name] = newStorage
	return nil
}

// UpdateStorage updates an existing Storage in the configuration.
func UpdateStorage(config *dto.Config, name string, updatedStorage *dto.Storage) error {
	_, found := config.Storage[name]
	if !found {
		return fmt.Errorf("storage %s not found", name)
	}
	if err := updatedStorage.Validate(); err != nil {
		return err
	}

	config.Storage[name] = updatedStorage
	return nil
}

// DeleteStorage deletes a Storage from the configuration if it is not used in any policy.
func DeleteStorage(config *dto.Config, name string) error {
	_, found := config.Storage[name]
	if !found {
		return fmt.Errorf("storage %s not found", name)
	}
	routine := util.Find(config.BackupRoutines, func(routine *dto.BackupRoutine) bool {
		return routine.Storage == name
	})
	if routine != nil {
		return fmt.Errorf("cannot delete storage as it is used in a routine %s", *routine)
	}

	delete(config.Storage, name)
	return nil
}
