//nolint:dupl
package service

import (
	"fmt"

	"github.com/aerospike/aerospike-backup-service/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/pkg/util"
)

// AddCluster adds a new AerospikeCluster to the configuration if a cluster with the same name
// doesn't already exist.
func AddCluster(config *dto.Config, name string, newCluster *dto.AerospikeCluster) error {
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
func UpdateCluster(config *dto.Config, name string, updatedCluster *dto.AerospikeCluster) error {
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
func DeleteCluster(config *dto.Config, name string) error {
	_, found := config.AerospikeClusters[name]
	if !found {
		return fmt.Errorf("cluster %s not found", name)
	}

	routine := util.Find(config.BackupRoutines, func(policy *dto.BackupRoutine) bool {
		return policy.SourceCluster == name
	})
	if routine != nil {
		return fmt.Errorf("cannot delete cluster as it is used in a routine %s", *routine)
	}

	delete(config.AerospikeClusters, name)
	return nil
}
