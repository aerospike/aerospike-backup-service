package service

import (
	"errors"
	"fmt"

	"github.com/aerospike/backup/pkg/model"
)

// AddRoutine
// adds a new BackupRoutine to the configuration if a routine with the same name doesn't already exist.
func AddRoutine(config *model.Config, newRoutine *model.BackupRoutine) error {
	if newRoutine.Storage == "" {
		return errors.New("storage is empty")
	}
	if newRoutine.SourceCluster == "" {
		return errors.New("cluster is empty")
	}
	_, found := config.Storage[newRoutine.Storage]
	if !found {
		return fmt.Errorf("storage %s not found", newRoutine.Storage)
	}
	_, found = config.AerospikeClusters[newRoutine.SourceCluster]
	if !found {
		return fmt.Errorf("cluster %s not found", newRoutine.SourceCluster)
	}
	_, found = config.BackupRoutines[newRoutine.Name]
	if found {
		return fmt.Errorf("aerospike routine with the same name %s already exists", newRoutine.Name)
	}

	config.BackupRoutines[newRoutine.Name] = newRoutine
	return nil
}

// UpdateRoutine
// updates an existing BackupRoutine in the configuration.
func UpdateRoutine(config *model.Config, updatedRoutine *model.BackupRoutine) error {
	_, found := config.BackupRoutines[updatedRoutine.Name]
	if !found {
		return fmt.Errorf("backup routine %s not found", updatedRoutine.Name)
	}

	config.BackupRoutines[updatedRoutine.Name] = updatedRoutine
	return nil
}

// DeleteRoutine
// deletes a BackupRoutine from the configuration.
func DeleteRoutine(config *model.Config, routineToDeleteName *string) error {
	_, found := config.BackupRoutines[*routineToDeleteName]
	if !found {
		return fmt.Errorf("backup routine %s not found", *routineToDeleteName)
	}

	delete(config.BackupRoutines, *routineToDeleteName)
	return nil
}
