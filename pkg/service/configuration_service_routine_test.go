package service

import (
	"testing"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aws/smithy-go/ptr"
)

func TestRoutine_Add(t *testing.T) {
	policy := "policy"
	cluster := "cluster"
	storage := "storage"
	routineName := "routine"
	config := &model.Config{
		BackupRoutines:    make(map[string]*model.BackupRoutine),
		BackupPolicies:    map[string]*model.BackupPolicy{policy: {}},
		AerospikeClusters: map[string]*model.AerospikeCluster{cluster: {}},
		Storage:           map[string]*model.Storage{storage: {}},
	}

	routine := model.BackupRoutine{Storage: storage, SourceCluster: cluster}
	err := AddRoutine(config, routineName, &routine)
	if err != nil {
		t.Errorf("AddRoutine failed, expected nil error, got %v", err)
	}
}

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
		{name: "existing", routine: model.BackupRoutine{Storage: storage, SourceCluster: cluster}},
		{name: "no storage", routine: model.BackupRoutine{SourceCluster: cluster}},
		{name: "no cluster", routine: model.BackupRoutine{Storage: storage}},
		{name: "wrong storage", routine: model.BackupRoutine{Storage: wrong, SourceCluster: cluster}},
		{name: "wrong cluster", routine: model.BackupRoutine{Storage: storage, SourceCluster: wrong}},
		{name: "existing policy", routine: model.BackupRoutine{}},
	}

	config := &model.Config{
		BackupRoutines:    map[string]*model.BackupRoutine{routine: {}},
		BackupPolicies:    map[string]*model.BackupPolicy{policy: {}},
		AerospikeClusters: map[string]*model.AerospikeCluster{cluster: {}},
		Storage:           map[string]*model.Storage{storage: {}},
	}

	for _, testRoutine := range fails {
		err := AddRoutine(config, routine, &testRoutine.routine)
		if err == nil {
			t.Errorf("Expected an error on %s", testRoutine.name)
		}
	}
}

func TestRoutine_Update(t *testing.T) {
	name := "routine1"
	name2 := "routine2"
	config := &model.Config{
		BackupRoutines: map[string]*model.BackupRoutine{name: {}},
	}

	updatedRoutine := &model.BackupRoutine{
		IntervalMillis: ptr.Int64(1000),
	}

	err := UpdateRoutine(config, name2, updatedRoutine)
	if err == nil {
		t.Errorf("UpdateRoutine failed, expected routine not found error")
	}

	err = UpdateRoutine(config, name, updatedRoutine)
	if err != nil {
		t.Errorf("UpdateRoutine failed, expected nil error, got %v", err)
	}

	if config.BackupRoutines[name].IntervalMillis != updatedRoutine.IntervalMillis {
		t.Errorf("UpdateRoutine failed, expected routine to be updated")
	}
}

func TestRoutine_Delete(t *testing.T) {
	name := "routine1"
	config := &model.Config{
		BackupRoutines: map[string]*model.BackupRoutine{name: {}},
	}

	err := DeleteRoutine(config, "routine2")
	if err == nil {
		t.Errorf("DeleteRoutine failed, expected nil error, got %v", err)
	}

	err = DeleteRoutine(config, name)
	if err != nil {
		t.Errorf("DeleteRoutine failed, expected nil error, got %v", err)
	}

	if len(config.BackupRoutines) != 0 {
		t.Errorf("DeleteRoutine failed, expected routine to be deleted, got %d",
			len(config.BackupRoutines))
	}
}
