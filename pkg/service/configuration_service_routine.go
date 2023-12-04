package service

import (
	"fmt"

	"github.com/aerospike/backup/pkg/model"
)

// AddRoutine
// adds a new BackupRoutine to the configuration if a routine with the same name doesn't already exist.
func AddRoutine(config *model.Config, name string, newRoutine *model.BackupRoutine) error {
	_, found := config.Storage[newRoutine.Storage]
	if !found {
		return fmt.Errorf("storage %s not found", newRoutine.Storage)
	}
	_, found = config.AerospikeClusters[newRoutine.SourceCluster]
	if !found {
		return fmt.Errorf("cluster %s not found", newRoutine.SourceCluster)
	}
	_, found = config.BackupRoutines[name]
	if found {
		return fmt.Errorf("aerospike routine with the same name %s already exists", name)
	}

	config.BackupRoutines[name] = newRoutine
	return nil
}

// UpdateRoutine
// updates an existing BackupRoutine in the configuration.
func UpdateRoutine(config *model.Config, name string, updatedRoutine *model.BackupRoutine) error {
	_, found := config.BackupRoutines[name]
	if !found {
		return fmt.Errorf("backup routine %s not found", name)
	}

	config.BackupRoutines[name] = updatedRoutine
	return nil
}

// DeleteRoutine
// deletes a BackupRoutine from the configuration.
func DeleteRoutine(config *model.Config, name string) error {
	_, found := config.BackupRoutines[name]
	if !found {
		return fmt.Errorf("backup routine %s not found", name)
	}

	delete(config.BackupRoutines, name)
	return nil
}
