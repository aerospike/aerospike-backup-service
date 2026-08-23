package service

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestConfigRetriever_RetrieveConfiguration_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	routine := &model.BackupRoutine{
		Name:    "test-routine",
		Storage: &model.LocalStorage{Path: t.TempDir()},
	}
	pathService := NewPathService(nil)
	operations := storage.NewOperations(storage.NewLocalStorageAccessor())

	backupCreated := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, content := range []string{"config-a", "config-b"} {
		filePath := pathService.GetConfigurationFilePath(routine.Name, backupCreated, i)
		require.NoError(t, operations.WriteDataFile(t.Context(), routine.Storage, filePath, []byte(content)))
	}

	backupReader := NewMockBackupReaderWriter(ctrl)
	backupReader.EXPECT().GetBackups(gomock.Any(), gomock.Any()).
		Return([]model.BackupDetails{{BackupMetadata: model.BackupMetadata{Created: backupCreated}}}, nil)

	retriever := NewConfigRetriever(backupReader, pathService, operations)
	data, err := retriever.RetrieveConfiguration(t.Context(), routine, time.Now())
	require.NoError(t, err)

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	require.Len(t, zr.File, 2)

	var allContent string
	var allContentSb62 strings.Builder
	for _, f := range zr.File {
		rc, err := f.Open()
		require.NoError(t, err)
		b, err := io.ReadAll(rc)
		require.NoError(t, err)
		require.NoError(t, rc.Close())
		allContentSb62.WriteString(string(b))
	}
	allContent += allContentSb62.String()
	assert.Contains(t, allContent, "config-a")
	assert.Contains(t, allContent, "config-b")
}

func TestConfigRetriever_RetrieveConfiguration_GetBackupsError(t *testing.T) {
	ctrl := gomock.NewController(t)

	routine := &model.BackupRoutine{Name: "routine-1", Storage: &model.LocalStorage{Path: "/data"}}
	backupReader := NewMockBackupReaderWriter(ctrl)
	backupsErr := errors.New("backend unavailable")
	backupReader.EXPECT().GetBackups(gomock.Any(), gomock.Any()).Return(nil, backupsErr)

	retriever := NewConfigRetriever(backupReader, NewPathService(nil), nil)
	_, err := retriever.RetrieveConfiguration(t.Context(), routine, time.Now())

	require.Error(t, err)
	assert.ErrorIs(t, err, backupsErr)
}

func TestConfigRetriever_RetrieveConfiguration_NoFullBackups(t *testing.T) {
	ctrl := gomock.NewController(t)

	routine := &model.BackupRoutine{Name: "routine-1", Storage: &model.LocalStorage{Path: "/data"}}
	backupReader := NewMockBackupReaderWriter(ctrl)
	backupReader.EXPECT().GetBackups(gomock.Any(), gomock.Any()).Return(nil, nil)

	retriever := NewConfigRetriever(backupReader, NewPathService(nil), nil)
	_, err := retriever.RetrieveConfiguration(t.Context(), routine, time.Now())

	require.Error(t, err)
	assert.ErrorIs(t, err, model.ErrNotFound)
}

func TestConfigRetriever_RetrieveConfiguration_ReadFilesError(t *testing.T) {
	ctrl := gomock.NewController(t)

	routine := &model.BackupRoutine{Name: "routine-1", Storage: &model.LocalStorage{Path: "/data"}}
	backupReader := NewMockBackupReaderWriter(ctrl)
	backupReader.EXPECT().GetBackups(gomock.Any(), gomock.Any()).
		Return([]model.BackupDetails{{BackupMetadata: model.BackupMetadata{Created: time.Now()}}}, nil)

	readErr := errors.New("read failed")
	operations := storage.NewMockOperations(ctrl)
	operations.EXPECT().
		ReadFiles(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, readErr)

	retriever := NewConfigRetriever(backupReader, NewPathService(nil), operations)
	_, err := retriever.RetrieveConfiguration(t.Context(), routine, time.Now())

	require.Error(t, err)
	assert.ErrorIs(t, err, readErr)
}

func TestConfigRetriever_RetrieveConfiguration_NoConfigFiles(t *testing.T) {
	ctrl := gomock.NewController(t)

	routine := &model.BackupRoutine{Name: "routine-1", Storage: &model.LocalStorage{Path: "/data"}}
	backupReader := NewMockBackupReaderWriter(ctrl)
	backupReader.EXPECT().GetBackups(gomock.Any(), gomock.Any()).
		Return([]model.BackupDetails{{BackupMetadata: model.BackupMetadata{Created: time.Now()}}}, nil)

	operations := storage.NewMockOperations(ctrl)
	operations.EXPECT().
		ReadFiles(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil)

	retriever := NewConfigRetriever(backupReader, NewPathService(nil), operations)
	_, err := retriever.RetrieveConfiguration(t.Context(), routine, time.Now())

	require.Error(t, err)
	assert.ErrorIs(t, err, model.ErrNotFound)
}

func TestPackageFiles(t *testing.T) {
	buffers := []*bytes.Buffer{
		bytes.NewBufferString("content-0"),
		bytes.NewBufferString("content-1"),
	}

	data, err := packageFiles(buffers)
	require.NoError(t, err)

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	require.Len(t, zr.File, 2)
	assert.Equal(t, "aerospike_0.conf", zr.File[0].Name)
	assert.Equal(t, "aerospike_1.conf", zr.File[1].Name)
}

func TestPackageFiles_Empty(t *testing.T) {
	data, err := packageFiles(nil)
	require.NoError(t, err)

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	assert.Empty(t, zr.File)
}
