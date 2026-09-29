package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
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

	writes := map[string][]byte{}
	operations := storage.NewMockOperations(ctrl)
	operations.EXPECT().
		WriteDataFile(gomock.Any(), routine.Storage, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ model.Storage, fileName string, content []byte) error {
			writes[fileName] = content
			return nil
		}).
		Times(len(infos))

	writer := NewClusterConfigWriter(pathService, operations, configSource)
	require.NoError(t, writer.Write(t.Context(), routine, timestamp))

	require.Len(t, writes, len(infos))
	for i, info := range infos {
		path := pathService.GetConfigurationFilePath(routine.Name, timestamp, i)
		require.Equal(t, []byte(info), writes[path])
	}
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

	operations := storage.NewMockOperations(ctrl)
	operations.EXPECT().
		WriteDataFile(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
