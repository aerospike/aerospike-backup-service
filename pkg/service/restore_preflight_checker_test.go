package service

import (
	"context"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

type fakeInfoGetter struct {
	backup.InfoGetter
}

func (f fakeInfoGetter) GetNamespacesList(ctx context.Context) ([]string, error) {
	return []string{"ns1"}, nil
}

func TestRestorePreflight_BlocksPathRestoreOnSameClusterNamespace(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	registry := NewMockRunningBackupsRegistry(ctrl)
	routines := NewMockroutineProvider(ctrl)

	clusterLabel := "cluster-a"
	cluster := &model.AerospikeCluster{ClusterLabel: &clusterLabel}
	backupRoutines := map[string]*model.BackupRoutine{
		"routine-1": {
			Name:          "routine-1",
			SourceCluster: cluster,
			Namespaces:    []string{"ns1"},
		},
	}
	routines.EXPECT().Routines().Return(backupRoutines)
	registry.EXPECT().
		GetRoutineState(backupRoutines["routine-1"]).
		Return(&model.RoutineState{Full: &model.RunningJob{}})

	preflight := NewRestorePreflight(registry, routines)

	infoGetter := fakeInfoGetter{}

	err := preflight.ValidatePathRestore(
		t.Context(),
		&model.RestoreRequest{
			DestinationCluster: *cluster,
		},
		infoGetter,
		[]model.BackupDetails{
			{BackupMetadata: model.BackupMetadata{Namespace: "ns1", FileCount: 1}},
		},
	)

	assert.ErrorContains(t, err,
		"restore not allowed during backups on routine routine-1 (cluster cluster-a, namespace \"ns1\")")
}

func TestRestorePreflight_BlocksTimeRestoreOnSameClusterNamespace(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	registry := NewMockRunningBackupsRegistry(ctrl)
	routines := NewMockroutineProvider(ctrl)

	clusterLabel := "cluster-a"
	cluster := &model.AerospikeCluster{ClusterLabel: &clusterLabel}
	backupRoutines := map[string]*model.BackupRoutine{
		"routine-1": {
			Name:          "routine-1",
			SourceCluster: cluster,
			Namespaces:    []string{"ns1"},
		},
	}
	routines.EXPECT().Routines().Return(backupRoutines)
	registry.EXPECT().
		GetRoutineState(backupRoutines["routine-1"]).
		Return(&model.RoutineState{Incremental: &model.RunningJob{}})

	preflight := NewRestorePreflight(registry, routines)

	infoGetter := fakeInfoGetter{}

	err := preflight.ValidateTimeRestore(
		t.Context(),
		&model.RestoreTimestampRequest{
			DestinationCluster: *cluster,
		},
		infoGetter,
		map[string][]model.BackupDetails{
			"ns1": {{BackupMetadata: model.BackupMetadata{Namespace: "ns1", FileCount: 1}}},
		},
	)

	assert.ErrorContains(t, err,
		"restore not allowed during backups on routine routine-1 (cluster cluster-a, namespace \"ns1\")")
}
