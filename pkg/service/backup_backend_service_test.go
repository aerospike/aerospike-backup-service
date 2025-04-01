package service

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalGetBackupsWithTimeFilters(t *testing.T) {
	service := setupLocalBackupBackendService(t)

	ctx := context.Background()
	routineName := "test-routine"

	// Create backups with different timestamps
	times := []time.Time{
		time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC),
		time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC),
		time.Date(2021, 1, 4, 0, 0, 0, 0, time.UTC),
		time.Date(2021, 1, 5, 0, 0, 0, 0, time.UTC),
	}

	for _, tm := range times {
		backupPath := getBackupPath(routineName, jobTypeFull, "test-ns", tm)

		metadata := model.BackupMetadata{
			Created:   tm,
			Namespace: "test-ns",
		}

		err := service.WriteBackupMetadata(ctx, routineName, backupPath, metadata)
		require.NoError(t, err)
	}

	// Expect all backups are returned without filters
	backups, err := service.GetBackups(ctx, NewFullBackupFilter(routineName))
	require.NoError(t, err)
	require.Len(t, backups, 5)
	assert.Equal(t, "test-routine/backup/1609459200000/data/test-ns", backups[0].Key)

	// Test FromTime filter
	fromFilter := NewFullBackupFilter(routineName).WithFromTime(times[2]) // From Jan 3
	fromBackups, err := service.GetBackups(ctx, fromFilter)
	require.NoError(t, err)
	require.Len(t, fromBackups, 3) // Should return Jan 3, 4, 5
	assert.Equal(t, times[2], fromBackups[0].Created)
	assert.Equal(t, times[3], fromBackups[1].Created)
	assert.Equal(t, times[4], fromBackups[2].Created)

	// Test ToTime filter
	toFilter := NewFullBackupFilter(routineName).WithToTime(times[2]) // Up to Jan 3
	toBackups, err := service.GetBackups(ctx, toFilter)
	require.NoError(t, err)
	require.Len(t, toBackups, 3) // Should return Jan 1, 2, 3
	assert.Equal(t, times[0], toBackups[0].Created)
	assert.Equal(t, times[1], toBackups[1].Created)
	assert.Equal(t, times[2], toBackups[2].Created)

	// Test both FromTime and ToTime
	rangeFilter := NewFullBackupFilter(routineName).
		WithFromTime(times[1]). // From Jan 2
		WithToTime(times[3]) // To Jan 4
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
	boundsFilter := NewFullBackupFilter(routineName).WithTimeBounds(timeBounds)
	boundsBackups, err := service.GetBackups(ctx, boundsFilter)
	require.NoError(t, err)
	require.Len(t, boundsBackups, 3) // Should return Jan 2, 3, 4
	assert.Equal(t, times[1], boundsBackups[0].Created)
	assert.Equal(t, times[2], boundsBackups[1].Created)
	assert.Equal(t, times[3], boundsBackups[2].Created)

	// Test Last() with time filters
	lastRangeFilter := NewFullBackupFilter(routineName).
		WithFromTime(times[1]). // From Jan 2
		WithToTime(times[3]). // To Jan 4
		Last()
	lastRangeBackups, err := service.GetBackups(ctx, lastRangeFilter)
	require.NoError(t, err)
	require.Len(t, lastRangeBackups, 1) // Should return only Jan 4
	assert.Equal(t, times[3], lastRangeBackups[0].Created)
}

func TestLocalGetBackups_RoutineNotFound(t *testing.T) {
	service := setupLocalBackupBackendService(t)

	ctx := context.Background()
	filter := NewFullBackupFilter("non-existent-routine")

	_, err := service.GetBackups(ctx, filter)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "routine not found")
}

func TestLocalDeleteBackup(t *testing.T) {
	service := setupLocalBackupBackendService(t)

	ctx := context.Background()
	routineName := "test-routine"

	// Create backup
	created := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	backupPath := getBackupPath(routineName, jobTypeFull, "test-ns", created)

	metadata := model.BackupMetadata{
		Created:   created,
		Namespace: "test-ns",
	}

	err := service.WriteBackupMetadata(ctx, routineName, backupPath, metadata)
	require.NoError(t, err)

	// Verify backup exists
	filter := NewFullBackupFilter(routineName)
	backups, err := service.GetBackups(ctx, filter)
	require.NoError(t, err)
	require.Len(t, backups, 1)

	// Delete backup
	err = service.Delete(ctx, routineName, backupPath)
	require.NoError(t, err)

	// Verify backup no longer exists
	backups, err = service.GetBackups(ctx, filter)
	require.NoError(t, err)
	assert.Len(t, backups, 0)
}

func TestWriteBackupMetadata_RoutineNotFound(t *testing.T) {
	service := setupLocalBackupBackendService(t)

	ctx := context.Background()
	routineName := "non-existent-routine"
	backupPath := getBackupPath(routineName, jobTypeFull, "test-ns", time.Now())
	metadata := model.BackupMetadata{
		Created:   time.Now(),
		Namespace: "test-ns",
	}

	// Attempt to write metadata
	err := service.WriteBackupMetadata(ctx, routineName, backupPath, metadata)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "routine not found")
}

func TestDelete_RoutineNotFound(t *testing.T) {
	service := setupLocalBackupBackendService(t)

	ctx := context.Background()
	routineName := "non-existent-routine"
	backupPath := getBackupPath(routineName, jobTypeFull, "test-ns", time.Now())

	// Attempt to delete backup
	err := service.Delete(ctx, routineName, backupPath)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "routine not found")
}

// Setup test helpers for local storage tests
func setupLocalBackupBackendService(t *testing.T) *BackupBackendServiceImpl {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "backup-test-*")
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = os.RemoveAll(tempDir)
	})

	config := model.NewConfig()
	routine := &model.BackupRoutine{
		Storage: &model.LocalStorage{
			Path: tempDir,
		},
	}

	err = config.AddRoutine("test-routine", routine)
	require.NoError(t, err)

	return NewBackupBackendService(config)
}
