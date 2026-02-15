package service

import (
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
			Finished:  tm,
			Namespace: testNamespace,
		}

		err := service.WriteBackupMetadata(t.Context(), routine, backupPath, metadata)
		require.NoError(t, err)
	}

	// Expect all backups are returned without filters
	backups, err := service.GetBackups(t.Context(), NewFullBackupFilter(routine))
	require.NoError(t, err)
	require.Len(t, backups, 5)
	assert.Equal(t, "test-routine/backup/1609459200000/data/test-ns", backups[0].Key)

	// Test FromTime filter
	fromFilter := NewFullBackupFilter(routine).WithFromTime(times[2]) // From Jan 3
	fromBackups, err := service.GetBackups(t.Context(), fromFilter)
	require.NoError(t, err)
	require.Len(t, fromBackups, 3) // Should return Jan 3, 4, 5
	assert.Equal(t, times[2], fromBackups[0].Created)
	assert.Equal(t, times[3], fromBackups[1].Created)
	assert.Equal(t, times[4], fromBackups[2].Created)

	// Test ToTime filter
	toFilter := NewFullBackupFilter(routine).WithToTime(times[2]) // Up to Jan 3
	toBackups, err := service.GetBackups(t.Context(), toFilter)
	require.NoError(t, err)
	require.Len(t, toBackups, 3) // Should return Jan 1, 2, 3
	assert.Equal(t, times[0], toBackups[0].Created)
	assert.Equal(t, times[1], toBackups[1].Created)
	assert.Equal(t, times[2], toBackups[2].Created)

	// Test both FromTime and ToTime
	rangeFilter := NewFullBackupFilter(routine).
		WithFromTime(times[1]). // From Jan 2
		WithToTime(times[3])    // To Jan 4
	rangeBackups, err := service.GetBackups(t.Context(), rangeFilter)
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
	boundsBackups, err := service.GetBackups(t.Context(), boundsFilter)
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
	lastRangeBackups, err := service.GetBackups(t.Context(), lastRangeFilter)
	require.NoError(t, err)
	require.Len(t, lastRangeBackups, 1) // Should return only Jan 4
	assert.Equal(t, times[3], lastRangeBackups[0].Created)
}

func TestWithToTime(t *testing.T) {
	service, pathService, routine := setupLocalBackupBackendService(t)

	created := time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC)
	toTime := created

	backupPath := pathService.GetBackupPath(routineName, jobTypeFull, testNamespace, created)
	metadata := model.BackupMetadata{
		Created:   created,
		Finished:  created.Add(1 * time.Hour), // finished after toTime
		Namespace: testNamespace,
	}
	err := service.WriteBackupMetadata(t.Context(), routine, backupPath, metadata)
	require.NoError(t, err)

	// WithToTime: backup must be finished by toTime; this one finishes later, so excluded
	backups, err := service.GetBackups(t.Context(), NewFullBackupFilter(routine).WithToTime(toTime.Add(10*time.Minute)))
	require.NoError(t, err)
	require.Empty(t, backups)

	// Same backup with Finished <= toTime is included
	metadata.Finished = created.Add(1 * time.Minute)
	err = service.WriteBackupMetadata(t.Context(), routine, backupPath, metadata)
	require.NoError(t, err)
	backups, err = service.GetBackups(t.Context(), NewFullBackupFilter(routine).WithToTime(toTime.Add(10*time.Minute)))
	require.NoError(t, err)
	require.Len(t, backups, 1)
	assert.Equal(t, created, backups[0].Created)
	assert.Equal(t, metadata.Finished, backups[0].Finished)
}

func TestWithTimeBounds(t *testing.T) {
	service, pathService, routine := setupLocalBackupBackendService(t)

	jan2 := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
	jan3 := time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC)
	jan4 := time.Date(2021, 1, 4, 0, 0, 0, 0, time.UTC)

	for _, tm := range []time.Time{jan2, jan3, jan4} {
		path := pathService.GetBackupPath(routineName, jobTypeFull, testNamespace, tm)
		err := service.WriteBackupMetadata(t.Context(), routine, path, model.BackupMetadata{
			Created:   tm,
			Finished:  tm,
			Namespace: testNamespace,
		})
		require.NoError(t, err)
	}

	// WithTimeBounds(FromTime=Jan2, ToTime=Jan4): Jan 2, 3, 4
	bounds := model.TimeBounds{FromTime: &jan2, ToTime: &jan4}
	backups, err := service.GetBackups(t.Context(), NewFullBackupFilter(routine).WithTimeBounds(bounds))
	require.NoError(t, err)
	require.Len(t, backups, 3)
	assert.Equal(t, jan2, backups[0].Created)
	assert.Equal(t, jan3, backups[1].Created)
	assert.Equal(t, jan4, backups[2].Created)

	// WithTimeBounds(FromTime=nil, ToTime=Jan3): Jan 2, 3
	boundsToOnly := model.TimeBounds{ToTime: &jan3}
	backups, err = service.GetBackups(t.Context(), NewFullBackupFilter(routine).WithTimeBounds(boundsToOnly))
	require.NoError(t, err)
	require.Len(t, backups, 2)
	assert.Equal(t, jan2, backups[0].Created)
	assert.Equal(t, jan3, backups[1].Created)

	// WithTimeBounds(FromTime=Jan3, ToTime=nil): Jan 3, 4
	boundsFromOnly := model.TimeBounds{FromTime: &jan3}
	backups, err = service.GetBackups(t.Context(), NewFullBackupFilter(routine).WithTimeBounds(boundsFromOnly))
	require.NoError(t, err)
	require.Len(t, backups, 2)
	assert.Equal(t, jan3, backups[0].Created)
	assert.Equal(t, jan4, backups[1].Created)

	// PathFilter with WithTimeBounds
	pathFilter := NewPathFilter("test-routine/backup", routine.Storage).WithTimeBounds(bounds)
	backups, err = service.GetBackups(t.Context(), pathFilter)
	require.NoError(t, err)
	require.Len(t, backups, 3)
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

	err := service.WriteBackupMetadata(t.Context(), routine, backupPath, metadata)
	require.NoError(t, err)

	// Verify backup exists
	filter := NewFullBackupFilter(routine)
	backups, err := service.GetBackups(t.Context(), filter)
	require.NoError(t, err)
	require.Len(t, backups, 1)

	// Delete backup
	err = service.Delete(t.Context(), routine, backupPath)
	require.NoError(t, err)

	// Verify backup no longer exists
	backups, err = service.GetBackups(t.Context(), filter)
	require.NoError(t, err)
	assert.Empty(t, backups)
}

func TestIncrementalBackup(t *testing.T) {
	service, pathService, routine := setupLocalBackupBackendService(t)

	// First create a full backup as baseline
	fullBackupTime := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	fullBackupPath := pathService.GetBackupPath(routineName, jobTypeFull, testNamespace, fullBackupTime)

	fullMetadata := model.BackupMetadata{
		Created:     fullBackupTime,
		Finished:    fullBackupTime,
		Namespace:   testNamespace,
		RecordCount: 1000,
		ByteCount:   10240,
		FileCount:   5,
	}

	err := service.WriteBackupMetadata(t.Context(), routine, fullBackupPath, fullMetadata)
	require.NoError(t, err)

	// Now create an incremental backup
	incrementalTime := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
	incrementalPath := pathService.GetBackupPath(routineName, jobTypeIncremental, testNamespace, incrementalTime)

	incMetadata := model.BackupMetadata{
		Created:     incrementalTime,
		Finished:    incrementalTime,
		From:        fullBackupTime,
		Namespace:   testNamespace,
		RecordCount: 200,
		ByteCount:   2048,
		FileCount:   2,
	}

	err = service.WriteBackupMetadata(t.Context(), routine, incrementalPath, incMetadata)
	require.NoError(t, err)

	// Test fetching only incremental backups
	filter := NewIncrementalBackupFilter(routine)
	backups, err := service.GetBackups(t.Context(), filter)

	// Verify
	require.NoError(t, err)
	require.Len(t, backups, 1)
	assert.Equal(t, uint64(200), backups[0].RecordCount)
	assert.Equal(t, incrementalTime, backups[0].Created)
	assert.Equal(t, fullBackupTime, backups[0].From)
	assert.Equal(t, "test-routine/incremental/1609545600000/data/test-ns", backups[0].Key)

	// Test with time filter for incremental
	timeFilter := NewIncrementalBackupFilter(routine).WithFromTime(fullBackupTime)
	timeBackups, err := service.GetBackups(t.Context(), timeFilter)
	require.NoError(t, err)
	require.Len(t, timeBackups, 1)
	assert.Equal(t, "test-routine/incremental/1609545600000/data/test-ns", timeBackups[0].Key)

	// Verify independent from full backups
	fullFilter := NewFullBackupFilter(routine)
	fullBackups, err := service.GetBackups(t.Context(), fullFilter)
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

	err := service.WriteBackupMetadata(t.Context(), routine, fullBackupPath, fullMetadata)
	require.NoError(t, err)

	// Now create an incremental backup
	incrementalTime := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
	incrementalPath := pathService.GetBackupPath(routineName, jobTypeIncremental, testNamespace, incrementalTime)

	incMetadata := model.BackupMetadata{
		Created: incrementalTime,
	}

	err = service.WriteBackupMetadata(t.Context(), routine, incrementalPath, incMetadata)
	require.NoError(t, err)

	backups, err := service.GetBackups(t.Context(), NewPathFilter("test-routine", routine.Storage))
	require.NoError(t, err)
	require.Len(t, backups, 2)

	backups, err = service.GetBackups(t.Context(), NewPathFilter("test-routine/backup", routine.Storage))
	require.NoError(t, err)
	require.Len(t, backups, 1)

	backups, err = service.GetBackups(t.Context(), NewPathFilter("test-routine/incremental", routine.Storage))
	require.NoError(t, err)
	require.Len(t, backups, 1)

	backups, err = service.GetBackups(t.Context(), NewPathFilter("test-routine/wrong-path", routine.Storage))
	require.NoError(t, err)
	require.Empty(t, backups)

	backups, err = service.GetBackups(t.Context(), NewPathFilter("wrong-path", routine.Storage))
	require.NoError(t, err)
	require.Empty(t, backups)
}

// Setup test helpers for local storage tests.
func setupLocalBackupBackendService(t *testing.T) (*BackupBackendServiceImpl, PathService, *model.BackupRoutine) {
	t.Helper()

	routine := &model.BackupRoutine{
		Name: "test-routine",
		Storage: &model.LocalStorage{
			Path: t.TempDir(),
		},
	}

	pathService := NewPathService(nil)
	return NewBackupBackendService(pathService,
		storage.NewOperations(storage.NewLocalStorageAccessor())), pathService, routine
}
