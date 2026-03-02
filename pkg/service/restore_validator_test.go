package service

import (
	"context"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type fakeInfoGetter struct {
	backup.InfoGetter
}

func (f fakeInfoGetter) GetNamespacesList(_ context.Context) ([]string, error) {
	return []string{"ns1"}, nil
}

func TestRestoreValidator_BlocksPathRestoreOnSameClusterNamespace(t *testing.T) {
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
		Return(model.RoutineState{Full: &model.RunningJob{}})

	validator := NewRestoreValidator(registry, routines)

	infoGetter := fakeInfoGetter{}

	err := validator.ValidatePath(
		t.Context(),
		&model.RestoreRequest{
			DestinationCluster: *cluster,
		},
		infoGetter,
		[]model.BackupDetails{
			{BackupMetadata: model.BackupMetadata{Namespace: "ns1", FileCount: 1}},
		},
	)

	require.ErrorContains(t, err,
		"restore not allowed during backups on routine routine-1 (cluster cluster-a, namespace \"ns1\")")
	require.ErrorIs(t, err, ErrRestorePrerequisitesFailed)
}

func TestRestoreValidator_BlocksTimeRestoreOnSameClusterNamespace(t *testing.T) {
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
		Return(model.RoutineState{Incremental: &model.RunningJob{}})

	validator := NewRestoreValidator(registry, routines)

	infoGetter := fakeInfoGetter{}

	err := validator.ValidateTimestamp(
		t.Context(),
		&model.RestoreTimestampRequest{
			DestinationCluster: *cluster,
		},
		infoGetter,
		map[string][]model.BackupDetails{
			"ns1": {{BackupMetadata: model.BackupMetadata{Namespace: "ns1", FileCount: 1}}},
		},
	)

	require.ErrorContains(t, err,
		"restore not allowed during backups on routine routine-1 (cluster cluster-a, namespace \"ns1\")")
	require.ErrorIs(t, err, ErrRestorePrerequisitesFailed)
}
