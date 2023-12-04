package service

import (
	"fmt"

	"github.com/aerospike/backup/pkg/model"
)

// AddPolicy
// adds a new BackupPolicy to the configuration if a policy with the same name doesn't already exist.
func AddPolicy(config *model.Config, name string, newPolicy *model.BackupPolicy) error {
	_, found := config.BackupPolicies[name]
	if found {
		return fmt.Errorf("backup policy with the same name %s already exists", name)
	}

	config.BackupPolicies[name] = newPolicy
	return nil
}

// UpdatePolicy
// updates an existing BackupPolicy in the configuration.
func UpdatePolicy(config *model.Config, name string, updatedPolicy *model.BackupPolicy) error {
	_, found := config.BackupPolicies[name]
	if !found {
		return fmt.Errorf("backup policy %s not found", name)
	}

	config.BackupPolicies[name] = updatedPolicy
	return nil
}

// DeletePolicy
// deletes a BackupPolicy from the configuration.
func DeletePolicy(config *model.Config, name string) error {
	_, found := config.BackupPolicies[name]
	if !found {
		return fmt.Errorf("backup policy %s not found", name)
	}

	delete(config.BackupPolicies, name)
	return nil
}
