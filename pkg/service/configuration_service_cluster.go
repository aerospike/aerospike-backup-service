//nolint:dupl
package service

import (
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/util"
)

// AddCluster adds a new AerospikeCluster to the configuration if a cluster with the same name
// doesn't already exist.
func AddCluster(config *model.Config, name string, newCluster *model.AerospikeCluster) error {
	_, found := config.AerospikeClusters[name]
	if found {
		return fmt.Errorf("aerospike cluster with the same name %s already exists", name)
	}
	if err := newCluster.Validate(); err != nil {
		return err
	}

	config.AerospikeClusters[name] = newCluster
	return nil
}

// UpdateCluster updates an existing AerospikeCluster in the configuration.
func UpdateCluster(config *model.Config, name string, updatedCluster *model.AerospikeCluster) error {
	_, found := config.AerospikeClusters[name]
	if !found {
		return fmt.Errorf("cluster %s not found", name)
	}
	if err := updatedCluster.Validate(); err != nil {
		return err
	}

	config.AerospikeClusters[name] = updatedCluster
	return nil
}

// DeleteCluster deletes an AerospikeCluster from the configuration if it is not used in
// any backup routine.
func DeleteCluster(config *model.Config, name string) error {
	_, found := config.AerospikeClusters[name]
	if !found {
		return fmt.Errorf("cluster %s not found", name)
	}

	routine := util.Find(config.BackupRoutines, func(policy *model.BackupRoutine) bool {
		return policy.SourceCluster == name
	})
	if routine != nil {
		return fmt.Errorf("cannot delete cluster as it is used in a routine %s", *routine)
	}

	delete(config.AerospikeClusters, name)
	return nil
}
