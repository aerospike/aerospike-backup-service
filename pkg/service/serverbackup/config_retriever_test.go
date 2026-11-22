package serverbackup

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/aerospike/backup-go/io/storage/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestConfigRetriever(t *testing.T) {
	t.Parallel()

	toTime := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	routine := &model.BackupRoutine{
		Name:    "routine",
		Storage: &model.S3Storage{Bucket: "bucket", S3Region: "us-east-1", Path: "backups"},
	}
	fullBackup := []model.BackupDetails{{Key: backupKey("routine", fullID)}}
	folder := "routine/" + fullID + "/config"
	files := map[string]string{
		folder + "/A1/aerospike.conf": "node A1",
		folder + "/B2/aerospike.conf": "node B2",
	}

	tests := []struct {
		name string
		// backups is the answer to the search for the last full backup.
		backups []model.BackupDetails
		// keys and keysErr are the answer to the listing of the backup's config folder.
		keys    []string
		keysErr error
		want    map[string]string
		wantErr string
	}{
		{
			name:    "a file per node",
			backups: fullBackup,
			keys:    []string{"backups/" + folder + "/A1/aerospike.conf", "backups/" + folder + "/B2/aerospike.conf"},
			want:    map[string]string{"A1/aerospike.conf": "node A1", "B2/aerospike.conf": "node B2"},
		},
		{name: "no full backup", wantErr: "no full backups found before"},
		{name: "backup without configuration files", backups: fullBackup,
			wantErr: "no configuration files found in " + folder},
		{name: "empty config folder", backups: fullBackup, keysErr: common.ErrEmptyStorage,
			wantErr: "no configuration files found in " + folder},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			backupReader := service.NewMockBackupReader(ctrl)
			backupReader.EXPECT().GetBackups(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, filter service.BackupFilter) ([]model.BackupDetails, error) {
					routineFilter := filter.(*service.RoutineFilter)
					assert.Equal(t, model.BackupTypeFull, routineFilter.BackupType())
					assert.True(t, routineFilter.OnlyLast())
					assert.Equal(t, &toTime, routineFilter.TimeBounds().ToTime)

					return tt.backups, nil
				})
			operations := storage.NewMockOperations(ctrl)
			operations.EXPECT().ReadFileNames(gomock.Any(), routine.Storage, folder, "", nil).
				Return(tt.keys, tt.keysErr).MaxTimes(1)
			operations.EXPECT().ReadFile(gomock.Any(), routine.Storage, gomock.Any()).DoAndReturn(
				func(_ context.Context, _ model.Storage, file string) ([]byte, error) {
					return []byte(files[file]), nil
				}).Times(len(tt.keys))

			archive, err := NewConfigRetriever(backupReader, operations).
				RetrieveConfiguration(t.Context(), routine, toTime)

			if tt.wantErr != "" {
				require.ErrorIs(t, err, model.ErrNotFound)
				require.ErrorContains(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, unzip(t, archive))
		})
	}
}

// TestConfigRetriever_ReadFails checks that a failed read of the storage is not reported as a
// backup without configuration files.
func TestConfigRetriever_ReadFails(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	routine := &model.BackupRoutine{Name: "routine", Storage: &model.S3Storage{Bucket: "bucket"}}
	backupReader := service.NewMockBackupReader(ctrl)
	backupReader.EXPECT().GetBackups(gomock.Any(), gomock.Any()).
		Return([]model.BackupDetails{{Key: backupKey("routine", fullID)}}, nil)
	operations := storage.NewMockOperations(ctrl)
	operations.EXPECT().ReadFileNames(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("storage down"))

	_, err := NewConfigRetriever(backupReader, operations).RetrieveConfiguration(t.Context(), routine, time.Now())

	require.EqualError(t, err,
		"failed to list the configuration files in routine/"+fullID+"/config: storage down")
	require.NotErrorIs(t, err, model.ErrNotFound)
}

// unzip returns the content of the files in archive, by name.
func unzip(t *testing.T, archive []byte) map[string]string {
	t.Helper()

	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	require.NoError(t, err)

	files := make(map[string]string, len(reader.File))
	for _, file := range reader.File {
		content, err := file.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(content)
		require.NoError(t, err)
		require.NoError(t, content.Close())

		files[file.Name] = string(data)
	}

	return files
}
