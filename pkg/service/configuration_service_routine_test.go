package service

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/internal/server/dto"
)

func TestRoutine_Add(t *testing.T) {
	policy := "policy"
	cluster := "cluster"
	storage := "storage"
	routineName := "routine"
	config := &dto.Config{
		BackupRoutines:    make(map[string]*dto.BackupRoutine),
		BackupPolicies:    map[string]*dto.BackupPolicy{policy: {}},
		AerospikeClusters: map[string]*dto.AerospikeCluster{cluster: {}},
		Storage:           map[string]*dto.Storage{storage: {}},
	}

	routine := dto.BackupRoutine{
		Storage:       storage,
		SourceCluster: cluster,
		BackupPolicy:  policy,
		IntervalCron:  "@daily",
	}
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
		routine dto.BackupRoutine
	}{
		{name: "empty", routine: dto.BackupRoutine{}},
		{name: "existing", routine: dto.BackupRoutine{Storage: storage, SourceCluster: cluster}},
		{name: "no storage", routine: dto.BackupRoutine{SourceCluster: cluster}},
		{name: "no cluster", routine: dto.BackupRoutine{Storage: storage}},
		{name: "wrong storage", routine: dto.BackupRoutine{Storage: wrong, SourceCluster: cluster}},
		{name: "wrong cluster", routine: dto.BackupRoutine{Storage: storage, SourceCluster: wrong}},
		{name: "existing policy", routine: dto.BackupRoutine{}},
	}

	config := &dto.Config{
		BackupRoutines:    map[string]*dto.BackupRoutine{routine: {}},
		BackupPolicies:    map[string]*dto.BackupPolicy{policy: {}},
		AerospikeClusters: map[string]*dto.AerospikeCluster{cluster: {}},
		Storage:           map[string]*dto.Storage{storage: {}},
	}

	for i := range fails {
		err := AddRoutine(config, routine, &fails[i].routine)
		if err == nil {
			t.Errorf("Expected an error on %s", fails[i].name)
		}
	}
}

func TestRoutine_Update(t *testing.T) {
	name := "routine1"
	name2 := "routine2"
	policy := "policy"
	cluster := "cluster"
	storage := "storage"
	config := &dto.Config{
		BackupRoutines:    map[string]*dto.BackupRoutine{name: {}},
		BackupPolicies:    map[string]*dto.BackupPolicy{policy: {}},
		AerospikeClusters: map[string]*dto.AerospikeCluster{cluster: {}},
		Storage:           map[string]*dto.Storage{storage: {}},
	}

	updatedRoutine := &dto.BackupRoutine{
		IntervalCron:  "* * * * * *",
		BackupPolicy:  policy,
		SourceCluster: cluster,
		Storage:       storage,
	}

	err := UpdateRoutine(config, name2, updatedRoutine)
	if err == nil {
		t.Errorf("UpdateRoutine failed, expected routine not found error")
	}

	err = UpdateRoutine(config, name, updatedRoutine)
	if err != nil {
		t.Errorf("UpdateRoutine failed, expected nil error, got %v", err)
	}

	if config.BackupRoutines[name].IntervalCron != updatedRoutine.IntervalCron {
		t.Errorf("UpdateRoutine failed, expected routine to be updated")
	}
}

func TestRoutine_Delete(t *testing.T) {
	name := "routine1"
	config := &dto.Config{
		BackupRoutines: map[string]*dto.BackupRoutine{name: {}},
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
