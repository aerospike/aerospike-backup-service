package service

import (
	"fmt"

	"github.com/aerospike/aerospike-backup-service/pkg/dto"
)

// AddRoutine adds a new BackupRoutine to the configuration if a routine with the same name
// doesn't already exist.
func AddRoutine(config *dto.Config, name string, newRoutine *dto.BackupRoutine) error {
	_, found := config.BackupRoutines[name]
	if found {
		return fmt.Errorf("aerospike routine with the same name %s already exists", name)
	}
	if err := newRoutine.Validate(config); err != nil {
		return err
	}

	config.BackupRoutines[name] = newRoutine
	return nil
}

// UpdateRoutine updates an existing BackupRoutine in the configuration.
func UpdateRoutine(config *dto.Config, name string, updatedRoutine *dto.BackupRoutine) error {
	_, found := config.BackupRoutines[name]
	if !found {
		return fmt.Errorf("backup routine %s not found", name)
	}
	if err := updatedRoutine.Validate(config); err != nil {
		return err
	}

	config.BackupRoutines[name] = updatedRoutine
	return nil
}

// DeleteRoutine deletes a BackupRoutine from the configuration.
func DeleteRoutine(config *dto.Config, name string) error {
	_, found := config.BackupRoutines[name]
	if !found {
		return fmt.Errorf("backup routine %s not found", name)
	}

	delete(config.BackupRoutines, name)
	return nil
}
