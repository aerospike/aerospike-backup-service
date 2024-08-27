package service

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/internal/server/dto"
	"github.com/aerospike/aerospike-backup-service/pkg/util"
)

func TestCluster_Add(t *testing.T) {
	name := "cluster1"
	config := &dto.Config{
		AerospikeClusters: map[string]*dto.AerospikeCluster{
			name: dto.NewLocalAerospikeCluster(),
		},
	}
	newCluster := dto.NewLocalAerospikeCluster()
	err := AddCluster(config, "cluster2", newCluster)
	if err != nil {
		t.Errorf("Error in adding cluster: %s", err.Error())
	}

	// Try adding the same cluster again, should return an error
	err = AddCluster(config, "cluster2", newCluster)
	if err == nil {
		t.Error("Expected an error while adding an existing cluster, but got nil")
	}
}

func TestCluster_Update(t *testing.T) {
	name := "cluster1"
	config := &dto.Config{
		AerospikeClusters: map[string]*dto.AerospikeCluster{
			name: dto.NewLocalAerospikeCluster(),
		},
	}
	updatedCluster := dto.NewLocalAerospikeCluster()
	if updatedCluster.Credentials == nil {
		updatedCluster.Credentials = &dto.Credentials{}
	}
	updatedCluster.Credentials.User = util.Ptr("user")
	err := UpdateCluster(config, name, updatedCluster)
	if err != nil {
		t.Errorf("Error in updating cluster: %s", err.Error())
	}
	if *config.AerospikeClusters[name].Credentials.User != "user" {
		t.Errorf("Value in cluster is not updated")
	}

	// Try updating a non-existent cluster, should return an error
	err = UpdateCluster(config, "cluster2", &dto.AerospikeCluster{})
	if err == nil {
		t.Error("Expected an error while updating a non-existent cluster, but got nil")
	}
}

func TestCluster_Delete(t *testing.T) {
	name := "cluster1"
	name2 := "cluster2"
	policy := "policy"
	routine := "routine"
	config := &dto.Config{
		AerospikeClusters: map[string]*dto.AerospikeCluster{name: {}, name2: {}},
		BackupPolicies:    map[string]*dto.BackupPolicy{policy: {}},
		BackupRoutines:    map[string]*dto.BackupRoutine{routine: {SourceCluster: name}},
	}
	err := DeleteCluster(config, name)
	if err == nil {
		t.Errorf("Expected an error while deleting cluster in use")
	}

	err = DeleteCluster(config, name2)
	if err != nil {
		t.Errorf("Error in deleting cluster: %s", err.Error())
	}

	// Try deleting a non-existent cluster, should return an error
	err = DeleteCluster(config, name2)
	if err == nil {
		t.Error("Expected an error while deleting a non-existent cluster, but got nil")
	}
}
