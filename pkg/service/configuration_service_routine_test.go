package service

import (
	"testing"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aws/smithy-go/ptr"
)

func TestRoutine_AddErrors(t *testing.T) {
	routine := "routine"
	policy := "policy"
	cluster := "cluster"
	storage := "storage"
	wrong := "-"
	fails := []struct {
		name    string
		routine model.BackupRoutine
	}{
		{name: "empty", routine: model.BackupRoutine{}},
		{name: "no storage", routine: model.BackupRoutine{SourceCluster: cluster}},
		{name: "no cluster", routine: model.BackupRoutine{Storage: storage}},
		{name: "wrong storage", routine: model.BackupRoutine{Storage: wrong, SourceCluster: cluster}},
		{name: "wrong cluster", routine: model.BackupRoutine{Storage: storage, SourceCluster: wrong}},
		{name: "existing policy", routine: model.BackupRoutine{Name: policy}},
	}

	config := &model.Config{
		BackupRoutines:    map[string]*model.BackupRoutine{routine: {Name: routine}},
		BackupPolicies:    map[string]*model.BackupPolicy{policy: {Name: &policy}},
		AerospikeClusters: map[string]*model.AerospikeCluster{cluster: {Name: &cluster}},
		Storage:           map[string]*model.Storage{storage: {Name: &storage}},
	}

	for _, testRoutine := range fails {
		err := AddRoutine(config, &testRoutine.routine)
		if err == nil {
			t.Errorf("Expected an error on %s", testRoutine.name)
		}
	}
}

func TestRoutine_Update(t *testing.T) {
	name := "routine1"
	config := &model.Config{
		BackupRoutines: map[string]*model.BackupRoutine{name: {Name: name}},
	}

	updatedRoutine := &model.BackupRoutine{
		Name: "routine2",
	}

	err := UpdateRoutine(config, updatedRoutine)
	if err == nil {
		t.Errorf("UpdateRoutine failed, expected routine not found error")
	}

	updatedRoutine.Name = name
	err = UpdateRoutine(config, updatedRoutine)
	if err != nil {
		t.Errorf("UpdateRoutine failed, expected nil error, got %v", err)
	}

	if config.BackupRoutines[name].Name != updatedRoutine.Name {
		t.Errorf("UpdateRoutine failed, expected routine name to be updated, got %v",
			config.BackupRoutines[name].Name)
	}
}

func TestRoutine_Delete(t *testing.T) {
	name := "routine1"
	config := &model.Config{
		BackupRoutines: map[string]*model.BackupRoutine{name: {Name: name}},
	}

	err := DeleteRoutine(config, ptr.String("routine2"))
	if err == nil {
		t.Errorf("DeleteRoutine failed, expected nil error, got %v", err)
	}

	err = DeleteRoutine(config, &name)
	if err != nil {
		t.Errorf("DeleteRoutine failed, expected nil error, got %v", err)
	}

	if len(config.BackupRoutines) != 0 {
		t.Errorf("DeleteRoutine failed, expected routine to be deleted, got %d",
			len(config.BackupRoutines))
	}
}
