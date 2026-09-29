package service

import (
	"context"
	"testing"
	"time"

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

	startController := NewMockStartController(ctrl)
	config := NewmockRoutineProvider(ctrl)

	clusterLabel := "cluster-a"
	cluster := &model.AerospikeCluster{ClusterLabel: clusterLabel}
	routines := map[string]*model.BackupRoutine{
		"routine-1": {
			Name:          "routine-1",
			SourceCluster: cluster,
			Namespaces:    []string{"ns1"},
		},
	}
	config.EXPECT().Routines().Return(routines)
	startController.EXPECT().
		HasBackupRunning(routines["routine-1"]).
		Return(true)

	validator := NewRestoreValidator(startController, config)

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

	startController := NewMockStartController(ctrl)
	config := NewmockRoutineProvider(ctrl)

	clusterLabel := "cluster-a"
	cluster := &model.AerospikeCluster{ClusterLabel: clusterLabel}
	routines := map[string]*model.BackupRoutine{
		"routine-1": {
			Name:          "routine-1",
			SourceCluster: cluster,
			Namespaces:    []string{"ns1"},
		},
	}
	config.EXPECT().Routines().Return(routines)
	startController.EXPECT().
		HasBackupRunning(routines["routine-1"]).
		Return(true)

	validator := NewRestoreValidator(startController, config)

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

func TestRestoreValidator_BlocksPathRestoreOnPendingBackupStart(t *testing.T) {
	ctrl := gomock.NewController(t)

	startController := NewMockStartController(ctrl)
	config := NewmockRoutineProvider(ctrl)

	clusterLabel := "cluster-a"
	cluster := &model.AerospikeCluster{ClusterLabel: clusterLabel}
	routines := map[string]*model.BackupRoutine{
		"routine-1": {
			Name:          "routine-1",
			SourceCluster: cluster,
			Namespaces:    []string{"ns1"},
		},
	}
	config.EXPECT().Routines().Return(routines)
	startController.EXPECT().
		HasBackupRunning(routines["routine-1"]).
		Return(true)

	validator := NewRestoreValidator(startController, config)

	err := validator.ValidatePath(
		t.Context(),
		&model.RestoreRequest{
			DestinationCluster: *cluster,
		},
		fakeInfoGetter{},
		[]model.BackupDetails{
			{BackupMetadata: model.BackupMetadata{Namespace: "ns1", FileCount: 1}},
		},
	)

	require.ErrorContains(t, err,
		"restore not allowed during backups on routine routine-1 (cluster cluster-a, namespace \"ns1\")")
	require.ErrorIs(t, err, ErrRestorePrerequisitesFailed)
}

func TestRestoreValidator_BlocksTimeRestoreOnPendingBackupStart(t *testing.T) {
	ctrl := gomock.NewController(t)

	startController := NewMockStartController(ctrl)
	config := NewmockRoutineProvider(ctrl)

	clusterLabel := "cluster-a"
	cluster := &model.AerospikeCluster{ClusterLabel: clusterLabel}
	routines := map[string]*model.BackupRoutine{
		"routine-1": {
			Name:          "routine-1",
			SourceCluster: cluster,
			Namespaces:    []string{"ns1"},
		},
	}
	config.EXPECT().Routines().Return(routines)
	startController.EXPECT().
		HasBackupRunning(routines["routine-1"]).
		Return(true)

	validator := NewRestoreValidator(startController, config)

	err := validator.ValidateTimestamp(
		t.Context(),
		&model.RestoreTimestampRequest{
			DestinationCluster: *cluster,
		},
		fakeInfoGetter{},
		map[string][]model.BackupDetails{
			"ns1": {{BackupMetadata: model.BackupMetadata{Namespace: "ns1", FileCount: 1}}},
		},
	)

	require.ErrorContains(t, err,
		"restore not allowed during backups on routine routine-1 (cluster cluster-a, namespace \"ns1\")")
	require.ErrorIs(t, err, ErrRestorePrerequisitesFailed)
}

func TestDestinationNamespacesForRestore(t *testing.T) {
	tests := map[string]struct {
		remapping        *model.RestoreNamespace
		sourceNamespaces []string
		want             []string
	}{
		"no remapping returns sources unchanged": {
			remapping:        nil,
			sourceNamespaces: []string{"ns1", "ns2"},
			want:             []string{"ns1", "ns2"},
		},
		"remapping maps every source to the same destination": {
			remapping:        &model.RestoreNamespace{Source: "ns1", Destination: "dest-ns"},
			sourceNamespaces: []string{"ns1", "ns2"},
			want:             []string{"dest-ns", "dest-ns"},
		},
		"no sources produces no destinations": {
			remapping:        &model.RestoreNamespace{Source: "ns1", Destination: "dest-ns"},
			sourceNamespaces: nil,
			want:             []string{},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := destinationNamespacesForRestore(tt.remapping, tt.sourceNamespaces)

			require.Equal(t, tt.want, got)
		})
	}
}

func TestValidateNamespaceRemapScope(t *testing.T) {
	tests := map[string]struct {
		remapping        *model.RestoreNamespace
		sourceNamespaces []string
		wantErr          string
	}{
		"no remapping configured": {
			remapping:        nil,
			sourceNamespaces: []string{"ns1", "ns2"},
		},
		"remapping scoped to exactly its source namespace": {
			remapping:        &model.RestoreNamespace{Source: "ns1", Destination: "dest-ns"},
			sourceNamespaces: []string{"ns1"},
		},
		"remapping scoped to its source namespace with duplicate entries": {
			remapping:        &model.RestoreNamespace{Source: "ns1", Destination: "dest-ns"},
			sourceNamespaces: []string{"ns1", "ns1"},
		},
		"remapping alongside a different namespace is rejected": {
			remapping:        &model.RestoreNamespace{Source: "ns1", Destination: "dest-ns"},
			sourceNamespaces: []string{"ns1", "ns2"},
			wantErr:          `namespace remap from "ns1" requires the restore to be scoped to exactly that namespace`,
		},
		"remapping a namespace other than its source is rejected": {
			remapping:        &model.RestoreNamespace{Source: "ns1", Destination: "dest-ns"},
			sourceNamespaces: []string{"ns2"},
			wantErr:          `namespace remap from "ns1" requires the restore to be scoped to exactly that namespace`,
		},
		"remapping with no namespaces in scope is rejected": {
			remapping:        &model.RestoreNamespace{Source: "ns1", Destination: "dest-ns"},
			sourceNamespaces: nil,
			wantErr:          `namespace remap from "ns1" requires the restore to be scoped to exactly that namespace`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := validateNamespaceRemapScope(tt.remapping, tt.sourceNamespaces)

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestRestoreValidator_BlocksPathRestoreOnNamespaceRemapOutOfScope(t *testing.T) {
	ctrl := gomock.NewController(t)

	startController := NewMockStartController(ctrl)
	config := NewmockRoutineProvider(ctrl)

	cluster := &model.AerospikeCluster{ClusterLabel: "cluster-a"}
	validator := NewRestoreValidator(startController, config)

	err := validator.ValidatePath(
		t.Context(),
		&model.RestoreRequest{
			DestinationCluster: *cluster,
			Policy: model.RestorePolicy{
				Namespace: &model.RestoreNamespace{Source: "ns1", Destination: "dest-ns"},
			},
		},
		fakeInfoGetter{},
		[]model.BackupDetails{
			{BackupMetadata: model.BackupMetadata{Namespace: "ns1", FileCount: 1}},
			{BackupMetadata: model.BackupMetadata{Namespace: "ns2", FileCount: 1}},
		},
	)

	require.ErrorContains(t, err,
		`namespace remap from "ns1" requires the restore to be scoped to exactly that namespace`)
	require.ErrorIs(t, err, ErrRestorePrerequisitesFailed)
}

func TestRestoreValidator_BlocksTimeRestoreOnNamespaceRemapOutOfScope(t *testing.T) {
	ctrl := gomock.NewController(t)

	startController := NewMockStartController(ctrl)
	config := NewmockRoutineProvider(ctrl)

	cluster := &model.AerospikeCluster{ClusterLabel: "cluster-a"}
	validator := NewRestoreValidator(startController, config)

	err := validator.ValidateTimestamp(
		t.Context(),
		&model.RestoreTimestampRequest{
			DestinationCluster: *cluster,
			Policy: model.RestorePolicy{
				Namespace: &model.RestoreNamespace{Source: "ns1", Destination: "dest-ns"},
			},
		},
		fakeInfoGetter{},
		map[string][]model.BackupDetails{
			"ns1": {{BackupMetadata: model.BackupMetadata{Namespace: "ns1", FileCount: 1}}},
			"ns2": {{BackupMetadata: model.BackupMetadata{Namespace: "ns2", FileCount: 1}}},
		},
	)

	require.ErrorContains(t, err,
		`namespace remap from "ns1" requires the restore to be scoped to exactly that namespace`)
	require.ErrorIs(t, err, ErrRestorePrerequisitesFailed)
}

func TestValidateNamespacesExist(t *testing.T) {
	tests := map[string]struct {
		destinationNamespaces []string
		clusterNamespaces     []string
		wantErr               string
	}{
		"all destination namespaces exist": {
			destinationNamespaces: []string{"ns1", "ns2"},
			clusterNamespaces:     []string{"ns1", "ns2", "ns3"},
		},
		"a destination namespace is missing": {
			destinationNamespaces: []string{"ns1", "dest-ns"},
			clusterNamespaces:     []string{"ns1"},
			wantErr:               "destination cluster does not have required namespace: dest-ns",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := validateNamespacesExist(tt.destinationNamespaces, tt.clusterNamespaces)

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestValidateNoRunningBackupConflict(t *testing.T) {
	cluster := model.AerospikeCluster{ClusterLabel: "cluster-a"}
	otherCluster := model.AerospikeCluster{ClusterLabel: "cluster-b"}

	tests := map[string]struct {
		destinationNamespaces []string
		running               []*model.BackupRoutine
		wantErr               string
	}{
		"no running backups": {
			destinationNamespaces: []string{"ns1"},
			running:               nil,
		},
		"running backup on a different cluster does not conflict": {
			destinationNamespaces: []string{"ns1"},
			running: []*model.BackupRoutine{
				{Name: "routine-1", SourceCluster: &otherCluster, Namespaces: []string{"ns1"}},
			},
		},
		"running backup on the same cluster with disjoint namespaces does not conflict": {
			destinationNamespaces: []string{"ns1"},
			running: []*model.BackupRoutine{
				{Name: "routine-1", SourceCluster: &cluster, Namespaces: []string{"ns2"}},
			},
		},
		"running backup on the same cluster and namespace conflicts": {
			destinationNamespaces: []string{"ns1"},
			running: []*model.BackupRoutine{
				{Name: "routine-1", SourceCluster: &cluster, Namespaces: []string{"ns1"}},
			},
			wantErr: `restore not allowed during backups on routine routine-1 (cluster cluster-a, namespace "ns1")`,
		},
		"running backup of the whole cluster conflicts with any restore into it": {
			destinationNamespaces: []string{"ns1"},
			running: []*model.BackupRoutine{
				{Name: "routine-1", SourceCluster: &cluster},
			},
			wantErr: `restore not allowed during backups on routine routine-1 (cluster cluster-a, all namespaces)`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := validateNoRunningBackupConflict(cluster, tt.destinationNamespaces, tt.running)

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestSourceNamespacesFromBackups(t *testing.T) {
	tests := map[string]struct {
		backups []model.BackupDetails
		want    []string
	}{
		"deduplicates namespaces": {
			backups: []model.BackupDetails{
				{BackupMetadata: model.BackupMetadata{Namespace: "ns1"}},
				{BackupMetadata: model.BackupMetadata{Namespace: "ns2"}},
				{BackupMetadata: model.BackupMetadata{Namespace: "ns1"}},
			},
			want: []string{"ns1", "ns2"},
		},
		"empty backups": {
			backups: nil,
			want:    []string{},
		},
		"single namespace": {
			backups: []model.BackupDetails{
				{BackupMetadata: model.BackupMetadata{Namespace: "ns1"}},
			},
			want: []string{"ns1"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := sourceNamespacesFromBackups(tt.backups)

			require.Equal(t, tt.want, got)
		})
	}
}

func TestValidateBackupsCreatedAtTheSameTime(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	later := now.Add(1 * time.Hour)

	tests := map[string]struct {
		backups []model.BackupDetails
		wantErr string
	}{
		"all backups at the same time": {
			backups: []model.BackupDetails{
				{BackupMetadata: model.BackupMetadata{Namespace: "ns1", Created: now}},
				{BackupMetadata: model.BackupMetadata{Namespace: "ns2", Created: now}},
			},
		},
		"backups at different times": {
			backups: []model.BackupDetails{
				{BackupMetadata: model.BackupMetadata{Namespace: "ns1", Created: now}},
				{BackupMetadata: model.BackupMetadata{Namespace: "ns2", Created: later}},
			},
			wantErr: "backup at index 1 created at",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := validateBackupsCreatedAtTheSameTime(tt.backups)

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}
