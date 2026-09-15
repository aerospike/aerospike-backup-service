package service

import (
	"errors"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	collectorRoutine = "routine1"
	fullRoot         = collectorRoutine + "/backup"
	incrementalRoot  = collectorRoutine + "/incremental"
)

const (
	// staleFolder is the name of a run that started well before partialBackupMinAge,
	// freshFolder of one that started within it.
	staleFolder = "1700000000000"
	freshFolder = "1799999000000"
)

// sweepNow is the moment the sweep runs in these tests.
var sweepNow = time.UnixMilli(1800000000000)

func collectorRoutines(routine *model.BackupRoutine) routineProvider {
	return &fixedRoutines{routines: map[string]*model.BackupRoutine{collectorRoutine: routine}}
}

type fixedRoutines struct {
	routines map[string]*model.BackupRoutine
}

func (f *fixedRoutines) Routines() map[string]*model.BackupRoutine {
	return f.routines
}

func newCollectorRoutine() *model.BackupRoutine {
	return &model.BackupRoutine{Name: collectorRoutine, Storage: &model.LocalStorage{Path: "/backups"}}
}

// A timestamp folder with no metadata file is the remains of a run that died with the process:
// retention never walks it, so the sweep is what removes it.
func TestPartialBackupCollector_RemovesFoldersWithoutMetadata(t *testing.T) {
	ctrl := gomock.NewController(t)
	routine := newCollectorRoutine()

	operations := storage.NewMockOperations(ctrl)
	operations.EXPECT().ReadFileNames(gomock.Any(), routine.Storage, fullRoot, "", nil).
		Return([]string{
			"/backups/" + fullRoot + "/" + staleFolder + "/data/ns1/0001.asb",
			"/backups/" + fullRoot + "/" + staleFolder + "/data/ns1/0002.asb",
		}, nil)
	operations.EXPECT().ReadFileNames(gomock.Any(), routine.Storage, incrementalRoot, "", nil).
		Return(nil, nil)

	catalog := NewMockBackupCatalog(ctrl)
	catalog.EXPECT().Delete(gomock.Any(), routine, fullRoot+"/"+staleFolder).Return(nil)

	collector := &partialBackupCollector{
		routines: collectorRoutines(routine), catalog: catalog, operations: operations,
	}
	collector.collect(t.Context(), sweepNow)
}

// A folder that has its metadata is a finished backup and belongs to retention, not the sweep.
func TestPartialBackupCollector_KeepsCompletedBackups(t *testing.T) {
	ctrl := gomock.NewController(t)
	routine := newCollectorRoutine()

	operations := storage.NewMockOperations(ctrl)
	operations.EXPECT().ReadFileNames(gomock.Any(), routine.Storage, fullRoot, "", nil).
		Return([]string{
			"/backups/" + fullRoot + "/" + staleFolder + "/data/ns1/0001.asb",
			"/backups/" + fullRoot + "/" + staleFolder + "/data/ns1/" + metadataFile,
		}, nil)
	operations.EXPECT().ReadFileNames(gomock.Any(), routine.Storage, incrementalRoot, "", nil).
		Return(nil, nil)

	catalog := NewMockBackupCatalog(ctrl) // no Delete: the backup is complete

	collector := &partialBackupCollector{
		routines: collectorRoutines(routine), catalog: catalog, operations: operations,
	}
	collector.collect(t.Context(), sweepNow)
}

// A folder younger than the minimum age may still be a backup in flight, so it is left alone.
func TestPartialBackupCollector_KeepsRecentFolders(t *testing.T) {
	ctrl := gomock.NewController(t)
	routine := newCollectorRoutine()

	operations := storage.NewMockOperations(ctrl)
	operations.EXPECT().ReadFileNames(gomock.Any(), routine.Storage, fullRoot, "", nil).
		Return([]string{"/backups/" + fullRoot + "/" + freshFolder + "/data/ns1/0001.asb"}, nil)
	operations.EXPECT().ReadFileNames(gomock.Any(), routine.Storage, incrementalRoot, "", nil).
		Return(nil, nil)

	catalog := NewMockBackupCatalog(ctrl) // no Delete: the run may still be writing

	collector := &partialBackupCollector{
		routines: collectorRoutines(routine), catalog: catalog, operations: operations,
	}
	collector.collect(t.Context(), sweepNow)
}

// Incremental folders are swept the same way as full ones.
func TestPartialBackupCollector_SweepsIncrementalFolders(t *testing.T) {
	ctrl := gomock.NewController(t)
	routine := newCollectorRoutine()

	operations := storage.NewMockOperations(ctrl)
	operations.EXPECT().ReadFileNames(gomock.Any(), routine.Storage, fullRoot, "", nil).
		Return(nil, nil)
	operations.EXPECT().ReadFileNames(gomock.Any(), routine.Storage, incrementalRoot, "", nil).
		Return([]string{"/backups/" + incrementalRoot + "/" + staleFolder + "/data/ns1/0001.asb"}, nil)

	catalog := NewMockBackupCatalog(ctrl)
	catalog.EXPECT().Delete(gomock.Any(), routine, incrementalRoot+"/"+staleFolder).Return(nil)

	collector := &partialBackupCollector{
		routines: collectorRoutines(routine), catalog: catalog, operations: operations,
	}
	collector.collect(t.Context(), sweepNow)
}

// A storage that cannot be listed leaves the other backup type, and the other routines, to be
// swept anyway.
func TestPartialBackupCollector_ContinuesAfterAListingFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	routine := newCollectorRoutine()

	operations := storage.NewMockOperations(ctrl)
	operations.EXPECT().ReadFileNames(gomock.Any(), routine.Storage, fullRoot, "", nil).
		Return(nil, errors.New("storage unreachable"))
	operations.EXPECT().ReadFileNames(gomock.Any(), routine.Storage, incrementalRoot, "", nil).
		Return([]string{"/backups/" + incrementalRoot + "/" + staleFolder + "/data/ns1/0001.asb"}, nil)

	catalog := NewMockBackupCatalog(ctrl)
	catalog.EXPECT().Delete(gomock.Any(), routine, incrementalRoot+"/"+staleFolder).Return(nil)

	collector := &partialBackupCollector{
		routines: collectorRoutines(routine), catalog: catalog, operations: operations,
	}
	require.Error(t, collector.collectRoutine(t.Context(), routine, sweepNow))
}

func TestTimestampFolderOf(t *testing.T) {
	tests := []struct {
		name       string
		file       string
		wantFolder string
		wantMillis int64
	}{
		{
			name:       "data file",
			file:       fullRoot + "/1700000000000/data/ns1/0001.asb",
			wantFolder: fullRoot + "/1700000000000",
			wantMillis: 1700000000000,
		},
		{
			name:       "formatted timestamp folder",
			file:       fullRoot + "/1700000000000_2023-11-14T22-13-20Z/data/ns1/0001.asb",
			wantFolder: fullRoot + "/1700000000000_2023-11-14T22-13-20Z",
			wantMillis: 1700000000000,
		},
		{name: "outside the root", file: "other/backup/1700000000000/data/ns1/0001.asb"},
		{name: "file directly in the root", file: fullRoot + "/stray.txt"},
		{name: "folder that is not a timestamp", file: fullRoot + "/notatimestamp/data/ns1/0001.asb"},
		{name: "timestamp too short", file: fullRoot + "/170000000/data/ns1/0001.asb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			folder, timestamp, ok := timestampFolderOf(tt.file, fullRoot)
			if tt.wantFolder == "" {
				assert.False(t, ok)
				return
			}

			require.True(t, ok)
			assert.Equal(t, tt.wantFolder, folder)
			assert.Equal(t, time.UnixMilli(tt.wantMillis), timestamp)
		})
	}
}
