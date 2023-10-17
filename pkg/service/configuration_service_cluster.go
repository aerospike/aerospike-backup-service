package service

import (
	"errors"
	"fmt"
	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/util"
)

// AddCluster
// adds a new AerospikeCluster to the configuration if a cluster with the same name doesn't already exist.
func AddCluster(config *model.Config, newCluster *model.AerospikeCluster) error {
	_, existing := util.GetByName(config.AerospikeClusters, newCluster.Name)
	if existing != nil {
		errorMessage := fmt.Sprintf("Aerospike cluster with the same name %s already exists", *newCluster.Name)
		return errors.New(errorMessage)
	}

	config.AerospikeClusters = append(config.AerospikeClusters, newCluster)
	return nil
}

// UpdateCluster
// updates an existing AerospikeCluster in the configuration
func UpdateCluster(config *model.Config, updatedCluster model.AerospikeCluster) error {
	i, existing := util.GetByName(config.AerospikeClusters, updatedCluster.Name)
	if existing != nil {
		config.AerospikeClusters[i] = &updatedCluster
		return nil
	}
	return errors.New(fmt.Sprintf("Cluster %s not found", *updatedCluster.Name))
}

// DeleteCluster
// deletes an AerospikeCluster from the configuration if it is not used in any policy
func DeleteCluster(config *model.Config, clusterToDeleteName string) error {
	for _, policy := range config.BackupPolicy {
		if *policy.SourceCluster == clusterToDeleteName {
			return errors.New(fmt.Sprintf("Cannot delete cluster as it is used in a policy %s", *policy.Name))
		}
	}

	i, existing := util.GetByName(config.AerospikeClusters, &clusterToDeleteName)
	if existing != nil {
		config.AerospikeClusters = append(config.AerospikeClusters[:i], config.AerospikeClusters[i+1:]...)
		return nil
	}
	return errors.New(fmt.Sprintf("Cluster %s not found", clusterToDeleteName))
}
