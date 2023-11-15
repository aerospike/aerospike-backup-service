package service

import (
	"errors"
	"fmt"

	"github.com/aerospike/backup/pkg/model"
)

// AddPolicy
// adds a new BackupPolicy to the configuration if a cluster with the same name doesn't already exist.
func AddPolicy(config *model.Config, newPolicy *model.BackupPolicy) error {
	if newPolicy.Storage == nil {
		return errors.New("storage is nil")
	}
	if newPolicy.SourceCluster == nil {
		return errors.New("cluster is nil")
	}
	_, found := config.BackupStorages[*newPolicy.Storage]
	if !found {
		return fmt.Errorf("storage %s not found", *newPolicy.Storage)
	}
	_, found = config.AerospikeClusters[*newPolicy.SourceCluster]
	if !found {
		return fmt.Errorf("cluster %s not found", *newPolicy.SourceCluster)
	}
	_, found = config.BackupPolicies[*newPolicy.Name]
	if found {
		return fmt.Errorf("aerospike policy with the same name %s already exists", *newPolicy.Name)
	}

	config.BackupPolicies[*newPolicy.Name] = newPolicy
	return nil
}

// UpdatePolicy
// updates an existing BackupPolicy in the configuration.
func UpdatePolicy(config *model.Config, updatedPolicy *model.BackupPolicy) error {
	_, found := config.BackupPolicies[*updatedPolicy.Name]
	if !found {
		return fmt.Errorf("policy %s not found", *updatedPolicy.Name)
	}

	config.BackupPolicies[*updatedPolicy.Name] = updatedPolicy
	return nil
}

// DeletePolicy
// deletes a BackupPolicy from the configuration.
func DeletePolicy(config *model.Config, policyToDeleteName *string) error {
	_, found := config.BackupPolicies[*policyToDeleteName]
	if !found {
		return fmt.Errorf("policy %s not found", *policyToDeleteName)
	}

	delete(config.BackupPolicies, *policyToDeleteName)
	return nil
}
