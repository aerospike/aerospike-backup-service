package service

import (
	"fmt"

	"github.com/aerospike/aerospike-backup-service/pkg/dto"
)

// AddPolicy adds a new BackupPolicy to the configuration if a policy with the same name
// doesn't already exist.
func AddPolicy(config *dto.Config, name string, newPolicy *dto.BackupPolicy) error {
	_, found := config.BackupPolicies[name]
	if found {
		return fmt.Errorf("backup policy with the same name %s already exists", name)
	}
	if err := newPolicy.Validate(); err != nil {
		return err
	}

	config.BackupPolicies[name] = newPolicy
	return nil
}

// UpdatePolicy updates an existing BackupPolicy in the configuration.
func UpdatePolicy(config *dto.Config, name string, updatedPolicy *dto.BackupPolicy) error {
	_, found := config.BackupPolicies[name]
	if !found {
		return fmt.Errorf("backup policy %s not found", name)
	}
	if err := updatedPolicy.Validate(); err != nil {
		return err
	}
	config.BackupPolicies[name] = updatedPolicy
	return nil
}

// DeletePolicy deletes a BackupPolicy from the configuration.
func DeletePolicy(config *dto.Config, name string) error {
	_, found := config.BackupPolicies[name]
	if !found {
		return fmt.Errorf("backup policy %s not found", name)
	}

	delete(config.BackupPolicies, name)
	return nil
}
