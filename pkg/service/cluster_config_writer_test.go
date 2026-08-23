package service

import (
	"errors"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestClusterConfigWriter_Write_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	routine := testClusterConfigRoutine()
	timestamp := time.UnixMilli(1_700_000_000_000)
	pathService := NewPathService(nil)
	infos := []asconfig.DotConf{"namespace ns1 {}", "namespace ns2 {}"}

	configSource := aerospike.NewMockClusterConfigSource(ctrl)
	configSource.EXPECT().
		NodeConfigs(gomock.Any(), routine.SourceCluster, gomock.Any()).
		Return(infos, nil)

	operations := NewmockStorageDataWriter(ctrl)
	for i, info := range infos {
		operations.EXPECT().
			WriteDataFile(
				gomock.Any(),
				routine.Storage,
				pathService.GetConfigurationFilePath(routine.Name, timestamp, i),
				[]byte(info),
			).
			Return(nil)
	}

	writer := NewClusterConfigWriter(pathService, operations, configSource)
	require.NoError(t, writer.Write(t.Context(), routine, timestamp))
}

func TestClusterConfigWriter_Write_SourceError(t *testing.T) {
	ctrl := gomock.NewController(t)

	routine := testClusterConfigRoutine()
	sourceErr := errors.New("failed to read Aerospike configuration")
	configSource := aerospike.NewMockClusterConfigSource(ctrl)
	configSource.EXPECT().
		NodeConfigs(gomock.Any(), routine.SourceCluster, gomock.Any()).
		Return(nil, sourceErr)

	writer := NewClusterConfigWriter(NewPathService(nil), nil, configSource)
	err := writer.Write(t.Context(), routine, time.Now())

	require.Error(t, err)
	require.ErrorIs(t, err, sourceErr)
}

func TestClusterConfigWriter_Write_StorageWriteError(t *testing.T) {
	ctrl := gomock.NewController(t)

	routine := testClusterConfigRoutine()
	timestamp := time.UnixMilli(1_700_000_000_000)
	pathService := NewPathService(nil)
	writeErr := errors.New("disk full")

	configSource := aerospike.NewMockClusterConfigSource(ctrl)
	configSource.EXPECT().
		NodeConfigs(gomock.Any(), routine.SourceCluster, gomock.Any()).
		Return([]asconfig.DotConf{"namespace ns1 {}"}, nil)

	operations := NewmockStorageDataWriter(ctrl)
	operations.EXPECT().
		WriteDataFile(
			gomock.Any(),
			routine.Storage,
			pathService.GetConfigurationFilePath(routine.Name, timestamp, 0),
			[]byte("namespace ns1 {}"),
		).
		Return(writeErr)

	writer := NewClusterConfigWriter(pathService, operations, configSource)
	err := writer.Write(t.Context(), routine, timestamp)

	require.Error(t, err)
	require.ErrorIs(t, err, writeErr)
	require.ErrorContains(t, err, "failed to write cluster configuration backup")
}

func testClusterConfigRoutine() *model.BackupRoutine {
	return &model.BackupRoutine{
		Name:          "routine-1",
		SourceCluster: &model.AerospikeCluster{},
		Storage:       &model.LocalStorage{Path: "/tmp/backups"},
	}
}
