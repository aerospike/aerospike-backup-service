package service

import (
	"context"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	routineName   = "test-routine"
	testNamespace = "test-ns"
)

var ctx = context.Background()

func TestLocalGetBackupsWithTimeFilters(t *testing.T) {
	service, pathService, routine := setupLocalBackupBackendService(t)

	// Create backups with different timestamps
	times := []time.Time{
		time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC),
		time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC),
		time.Date(2021, 1, 4, 0, 0, 0, 0, time.UTC),
		time.Date(2021, 1, 5, 0, 0, 0, 0, time.UTC),
	}

	for _, tm := range times {
		backupPath := pathService.GetBackupPath(routineName, jobTypeFull, testNamespace, tm)

		metadata := model.BackupMetadata{
			Created:   tm,
			Namespace: testNamespace,
		}

		err := service.WriteBackupMetadata(ctx, routine, backupPath, metadata)
		require.NoError(t, err)
	}

	// Expect all backups are returned without filters
	backups, err := service.GetBackups(ctx, NewFullBackupFilter(routine))
	require.NoError(t, err)
	require.Len(t, backups, 5)
	assert.Equal(t, "test-routine/backup/1609459200000/data/test-ns", backups[0].Key)

	// Test FromTime filter
	fromFilter := NewFullBackupFilter(routine).WithFromTime(times[2]) // From Jan 3
	fromBackups, err := service.GetBackups(ctx, fromFilter)
	require.NoError(t, err)
	require.Len(t, fromBackups, 3) // Should return Jan 3, 4, 5
	assert.Equal(t, times[2], fromBackups[0].Created)
	assert.Equal(t, times[3], fromBackups[1].Created)
	assert.Equal(t, times[4], fromBackups[2].Created)

	// Test ToTime filter
	toFilter := NewFullBackupFilter(routine).WithToTime(times[2]) // Up to Jan 3
	toBackups, err := service.GetBackups(ctx, toFilter)
	require.NoError(t, err)
	require.Len(t, toBackups, 3) // Should return Jan 1, 2, 3
	assert.Equal(t, times[0], toBackups[0].Created)
	assert.Equal(t, times[1], toBackups[1].Created)
	assert.Equal(t, times[2], toBackups[2].Created)

	// Test both FromTime and ToTime
	rangeFilter := NewFullBackupFilter(routine).
		WithFromTime(times[1]). // From Jan 2
		WithToTime(times[3])    // To Jan 4
	rangeBackups, err := service.GetBackups(ctx, rangeFilter)
	require.NoError(t, err)
	require.Len(t, rangeBackups, 3) // Should return Jan 2, 3, 4
	assert.Equal(t, times[1], rangeBackups[0].Created)
	assert.Equal(t, times[2], rangeBackups[1].Created)
	assert.Equal(t, times[3], rangeBackups[2].Created)

	// Test TimeBounds
	timeBounds := model.TimeBounds{
		FromTime: &times[1], // From Jan 2
		ToTime:   &times[3], // To Jan 4
	}
	boundsFilter := NewFullBackupFilter(routine).WithTimeBounds(timeBounds)
	boundsBackups, err := service.GetBackups(ctx, boundsFilter)
	require.NoError(t, err)
	require.Len(t, boundsBackups, 3) // Should return Jan 2, 3, 4
	assert.Equal(t, times[1], boundsBackups[0].Created)
	assert.Equal(t, times[2], boundsBackups[1].Created)
	assert.Equal(t, times[3], boundsBackups[2].Created)

	// Test Last() with time filters
	lastRangeFilter := NewFullBackupFilter(routine).
		WithFromTime(times[1]). // From Jan 2
		WithToTime(times[3]).   // To Jan 4
		Last()
	lastRangeBackups, err := service.GetBackups(ctx, lastRangeFilter)
	require.NoError(t, err)
	require.Len(t, lastRangeBackups, 1) // Should return only Jan 4
	assert.Equal(t, times[3], lastRangeBackups[0].Created)
}

func TestLocalDeleteBackup(t *testing.T) {
	service, pathService, routine := setupLocalBackupBackendService(t)

	// Create backup
	created := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	backupPath := pathService.GetBackupPath(routineName, jobTypeFull, testNamespace, created)

	metadata := model.BackupMetadata{
		Created:   created,
		Namespace: testNamespace,
	}

	err := service.WriteBackupMetadata(ctx, routine, backupPath, metadata)
	require.NoError(t, err)

	// Verify backup exists
	filter := NewFullBackupFilter(routine)
	backups, err := service.GetBackups(ctx, filter)
	require.NoError(t, err)
	require.Len(t, backups, 1)

	// Delete backup
	err = service.Delete(ctx, routine, backupPath)
	require.NoError(t, err)

	// Verify backup no longer exists
	backups, err = service.GetBackups(ctx, filter)
	require.NoError(t, err)
	assert.Len(t, backups, 0)
}

func TestIncrementalBackup(t *testing.T) {
	service, pathService, routine := setupLocalBackupBackendService(t)

	// First create a full backup as baseline
	fullBackupTime := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	fullBackupPath := pathService.GetBackupPath(routineName, jobTypeFull, testNamespace, fullBackupTime)

	fullMetadata := model.BackupMetadata{
		Created:     fullBackupTime,
		Namespace:   testNamespace,
		RecordCount: 1000,
		ByteCount:   10240,
		FileCount:   5,
	}

	err := service.WriteBackupMetadata(ctx, routine, fullBackupPath, fullMetadata)
	require.NoError(t, err)

	// Now create an incremental backup
	incrementalTime := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
	incrementalPath := pathService.GetBackupPath(routineName, jobTypeIncremental, testNamespace, incrementalTime)

	incMetadata := model.BackupMetadata{
		Created:     incrementalTime,
		From:        fullBackupTime, // From points to the full backup time
		Namespace:   testNamespace,
		RecordCount: 200,
		ByteCount:   2048,
		FileCount:   2,
	}

	err = service.WriteBackupMetadata(ctx, routine, incrementalPath, incMetadata)
	require.NoError(t, err)

	// Test fetching only incremental backups
	filter := NewIncrementalBackupFilter(routine)
	backups, err := service.GetBackups(ctx, filter)

	// Verify
	require.NoError(t, err)
	require.Len(t, backups, 1)
	assert.Equal(t, uint64(200), backups[0].RecordCount)
	assert.Equal(t, incrementalTime, backups[0].Created)
	assert.Equal(t, fullBackupTime, backups[0].From)
	assert.Equal(t, "test-routine/incremental/1609545600000/data/test-ns", backups[0].Key)

	// Test with time filter for incremental
	timeFilter := NewIncrementalBackupFilter(routine).WithFromTime(fullBackupTime)
	timeBackups, err := service.GetBackups(ctx, timeFilter)
	require.NoError(t, err)
	require.Len(t, timeBackups, 1)
	assert.Equal(t, "test-routine/incremental/1609545600000/data/test-ns", timeBackups[0].Key)

	// Verify independent from full backups
	fullFilter := NewFullBackupFilter(routine)
	fullBackups, err := service.GetBackups(ctx, fullFilter)
	require.NoError(t, err)
	require.Len(t, fullBackups, 1)
	assert.Equal(t, fullBackupTime, fullBackups[0].Created)
	assert.Equal(t, "test-routine/backup/1609459200000/data/test-ns", fullBackups[0].Key)
}

func TestReadPath(t *testing.T) {
	service, pathService, routine := setupLocalBackupBackendService(t)

	// First create a full backup as baseline
	fullBackupTime := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	fullBackupPath := pathService.GetBackupPath(routineName, jobTypeFull, testNamespace, fullBackupTime)

	fullMetadata := model.BackupMetadata{
		Created: fullBackupTime,
	}

	err := service.WriteBackupMetadata(ctx, routine, fullBackupPath, fullMetadata)
	require.NoError(t, err)

	// Now create an incremental backup
	incrementalTime := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
	incrementalPath := pathService.GetBackupPath(routineName, jobTypeIncremental, testNamespace, incrementalTime)

	incMetadata := model.BackupMetadata{
		Created: incrementalTime,
	}

	err = service.WriteBackupMetadata(ctx, routine, incrementalPath, incMetadata)
	require.NoError(t, err)

	backups, err := service.GetBackups(ctx, NewPathFilter("test-routine", routine.Storage))
	require.NoError(t, err)
	require.Len(t, backups, 2)

	backups, err = service.GetBackups(ctx, NewPathFilter("test-routine/backup", routine.Storage))
	require.NoError(t, err)
	require.Len(t, backups, 1)

	backups, err = service.GetBackups(ctx, NewPathFilter("test-routine/incremental", routine.Storage))
	require.NoError(t, err)
	require.Len(t, backups, 1)

	backups, err = service.GetBackups(ctx, NewPathFilter("test-routine/wrong-path", routine.Storage))
	require.NoError(t, err)
	require.Empty(t, backups)

	backups, err = service.GetBackups(ctx, NewPathFilter("wrong-path", routine.Storage))
	require.NoError(t, err)
	require.Empty(t, backups)
}

// Setup test helpers for local storage tests.
func setupLocalBackupBackendService(t *testing.T) (*BackupBackendServiceImpl, PathService, *model.BackupRoutine) {
	t.Helper()

	tempDir := t.TempDir()

	config := model.NewConfig()
	routine := &model.BackupRoutine{
		Name: "test-routine",
		Storage: &model.LocalStorage{
			Path: tempDir,
		},
	}

	err := config.AddRoutine(routine)
	require.NoError(t, err)

	pathService := NewPathService(nil)
	return NewBackupBackendService(config, pathService,
		storage.NewOperations(storage.NewLocalStorageAccessor())), pathService, routine
}
