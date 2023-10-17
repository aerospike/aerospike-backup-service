package service

import (
	"github.com/aerospike/backup/pkg/model"
	"github.com/aws/smithy-go/ptr"
	"testing"
)

func TestAddCluster(t *testing.T) {
	config := &model.Config{
		AerospikeClusters: []*model.AerospikeCluster{{Name: ptr.String("cluster1")}},
	}
	newCluster := &model.AerospikeCluster{Name: ptr.String("cluster2")}
	err := AddCluster(config, newCluster)
	if err != nil {
		t.Errorf("Error in adding cluster: %s", err.Error())
	}

	// Try adding the same cluster again, should return an error
	err = AddCluster(config, newCluster)
	if err == nil {
		t.Error("Expected an error while adding an existing cluster, but got nil")
	}
}

func TestUpdateCluster(t *testing.T) {
	name := "cluster1"
	config := &model.Config{
		AerospikeClusters: []*model.AerospikeCluster{{Name: &name}},
	}
	updatedCluster := &model.AerospikeCluster{Name: &name, User: ptr.String("user")}
	err := UpdateCluster(config, updatedCluster)
	if err != nil {
		t.Errorf("Error in updating cluster: %s", err.Error())
	}
	if *config.AerospikeClusters[0].User != "user" {
		t.Errorf("Value in cluster is not updated")
	}

	// Try updating a non-existent cluster, should return an error
	updatedCluster = &model.AerospikeCluster{Name: ptr.String("cluster2")}
	err = UpdateCluster(config, updatedCluster)
	if err == nil {
		t.Error("Expected an error while updating a non-existent cluster, but got nil")
	}
}

func TestDeleteCluster(t *testing.T) {
	name := ptr.String("cluster1")
	name2 := ptr.String("cluster2")
	config := &model.Config{
		AerospikeClusters: []*model.AerospikeCluster{{Name: name}, {Name: name2}},
		BackupPolicy:      []*model.BackupPolicy{{Name: ptr.String("policy1"), SourceCluster: name}},
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
