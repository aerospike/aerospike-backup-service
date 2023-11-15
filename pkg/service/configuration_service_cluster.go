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
	_, found := config.AerospikeClusters[*newCluster.Name]
	if found {
		return fmt.Errorf("aerospike cluster with the same name %s already exists", *newCluster.Name)
	}

	config.AerospikeClusters[*newCluster.Name] = newCluster
	return nil
}

// UpdateCluster
// updates an existing AerospikeCluster in the configuration.
func UpdateCluster(config *model.Config, updatedCluster *model.AerospikeCluster) error {
	_, found := config.AerospikeClusters[*updatedCluster.Name]
	if !found {
		return fmt.Errorf("cluster %s not found", *updatedCluster.Name)
	}

	config.AerospikeClusters[*updatedCluster.Name] = updatedCluster
	return nil
}

// DeleteCluster
// deletes an AerospikeCluster from the configuration if it is not used in any policy.
func DeleteCluster(config *model.Config, clusterToDeleteName *string) error {
	_, found := config.AerospikeClusters[*clusterToDeleteName]
	if !found {
		return fmt.Errorf("cluster %s not found", *clusterToDeleteName)
	}

	policy, found := util.Find(config.BackupPolicy, func(policy *model.BackupPolicy) bool {
		return *policy.SourceCluster == *clusterToDeleteName
	})
	if found {
		return fmt.Errorf("cannot delete cluster as it is used in a policy %s", *policy.Name)
	}

	delete(config.AerospikeClusters, *clusterToDeleteName)
	return nil
}
