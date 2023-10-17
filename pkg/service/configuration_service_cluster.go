//nolint:dupl
package service

import (
	"fmt"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/util"
)

// AddCluster
// adds a new AerospikeCluster to the configuration if a cluster with the same name doesn't already exist.
func AddCluster(config *model.Config, newCluster *model.AerospikeCluster) error {
	_, existing := util.GetByName(config.AerospikeClusters, newCluster.Name)
	if existing != nil {
		return fmt.Errorf("aerospike cluster with the same name %s already exists", *newCluster.Name)
	}

	config.AerospikeClusters = append(config.AerospikeClusters, newCluster)
	return nil
}

// UpdateCluster
// updates an existing AerospikeCluster in the configuration.
func UpdateCluster(config *model.Config, updatedCluster *model.AerospikeCluster) error {
	i, existing := util.GetByName(config.AerospikeClusters, updatedCluster.Name)
	if existing != nil {
		config.AerospikeClusters[i] = updatedCluster
		return nil
	}
	return fmt.Errorf("cluster %s not found", *updatedCluster.Name)
}

// DeleteCluster
// deletes an AerospikeCluster from the configuration if it is not used in any policy.
func DeleteCluster(config *model.Config, clusterToDeleteName *string) error {
	_, policy := util.Find(config.BackupPolicy, func(policy *model.BackupPolicy) bool {
		return *policy.SourceCluster == *clusterToDeleteName
	})

	if policy != nil {
		return fmt.Errorf("cannot delete cluster as it is used in a policy %s", *policy.Name)
	}

	i, existing := util.GetByName(config.AerospikeClusters, clusterToDeleteName)
	if existing != nil {
		config.AerospikeClusters = append(config.AerospikeClusters[:i], config.AerospikeClusters[i+1:]...)
		return nil
	}
	return fmt.Errorf("cluster %s not found", *clusterToDeleteName)
}
