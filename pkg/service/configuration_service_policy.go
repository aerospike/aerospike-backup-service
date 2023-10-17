package service

import (
	"errors"
	"fmt"
	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/util"
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
	_, storage := util.GetByName(config.BackupStorage, newPolicy.Storage)
	if storage == nil {
		return fmt.Errorf("storage %s not found", *newPolicy.Storage)
	}

	_, cluster := util.GetByName(config.AerospikeClusters, newPolicy.SourceCluster)
	if cluster == nil {
		return fmt.Errorf("cluster %s not found", *newPolicy.SourceCluster)
	}
	_, existing := util.GetByName(config.BackupPolicy, newPolicy.Name)
	if existing != nil {
		return fmt.Errorf("aerospike policy with the same name %s already exists", *newPolicy.Name)
	}

	config.BackupPolicy = append(config.BackupPolicy, newPolicy)
	return nil
}

// UpdatePolicy
// updates an existing BackupPolicy in the configuration
func UpdatePolicy(config *model.Config, updatedPolicy *model.BackupPolicy) error {
	i, existing := util.GetByName(config.BackupPolicy, updatedPolicy.Name)
	if existing != nil {
		config.BackupPolicy[i] = updatedPolicy
		return nil
	}

	return fmt.Errorf("policy %s not found", *updatedPolicy.Name)
}

// DeletePolicy
// deletes an BackupPolicy from the configuration if it is not used in any policy
func DeletePolicy(config *model.Config, policyToDeleteName *string) error {
	i, existing := util.GetByName(config.BackupPolicy, policyToDeleteName)
	if existing != nil {
		config.BackupPolicy = append(config.BackupPolicy[:i], config.BackupPolicy[i+1:]...)
		return nil
	}

	return fmt.Errorf("policy %s not found", *policyToDeleteName)
}
