package service

import (
	"errors"
	"fmt"
	"github.com/aerospike/backup/pkg/model"
)

// AddPolicy
// adds a new BackupPolicy to the configuration if a cluster with the same name doesn't already exist.
func AddPolicy(config *model.Config, newPolicy *model.BackupPolicy) error {
	for _, existingPolicy := range config.BackupPolicy {
		if *existingPolicy.Name == *newPolicy.Name {
			errorMessage := fmt.Sprintf("Aerospike policy with the same name %s already exists", *newPolicy.Name)
			return errors.New(errorMessage)
		}
	}

	config.BackupPolicy = append(config.BackupPolicy, newPolicy)
	return nil
}

// UpdatePolicy
// updates an existing BackupPolicy in the configuration
func UpdatePolicy(config *model.Config, updatedPolicy model.BackupPolicy) error {
	for i, cluster := range config.BackupPolicy {
		if *cluster.Name == *updatedPolicy.Name {
			config.BackupPolicy[i] = &updatedPolicy
			return nil
		}
	}
	return errors.New(fmt.Sprintf("Policy %s not found", *updatedPolicy.Name))
}

// DeletePolicy
// deletes an BackupPolicy from the configuration if it is not used in any policy
func DeletePolicy(config *model.Config, policyToDeleteName string) error {
	for i, cluster := range config.BackupPolicy {
		if *cluster.Name == policyToDeleteName {
			config.BackupPolicy = append(config.BackupPolicy[:i], config.BackupPolicy[i+1:]...)
			return nil
		}
	}
	return errors.New(fmt.Sprintf("Policy %s not found", policyToDeleteName))
}
