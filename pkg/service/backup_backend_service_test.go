package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Setup test helpers for local storage tests
func setupLocalBackupBackendService(t *testing.T) (*BackupBackendServiceImpl, *model.Config, string) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "backup-test-*")
	require.NoError(t, err)

	// Clean up the temporary directory after the test
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	config := model.NewConfig()
	localStorage := &model.LocalStorage{
		Path: tempDir,
	}

	routine := &model.BackupRoutine{
		Storage: localStorage,
	}

	err = config.AddRoutine("test-routine", routine)
	require.NoError(t, err)

	service := NewBackupBackendService(config)
	return service, config, tempDir
}

func TestLocalGetBackups_RoutineNotFound(t *testing.T) {
	// Setup
	service, _, _ := setupLocalBackupBackendService(t)

	ctx := context.Background()
	filter := NewFullBackupFilter("non-existent-routine")

	// Execute
	backups, err := service.GetBackups(ctx, filter)

	// Verify
	assert.Error(t, err)
	assert.Nil(t, backups)
	assert.Contains(t, err.Error(), "routine not found")
}

func TestLocalWriteAndGetBackupMetadata(t *testing.T) {
	// Setup
	service, _, tempDir := setupLocalBackupBackendService(t)

	ctx := context.Background()
	routineName := "test-routine"

	// Create directory structure for the backup
	backupPath := filepath.Join(routineName, "backup", "1609459200000", "data", "test-ns")
	fullPath := filepath.Join(tempDir, backupPath)
	err := os.MkdirAll(fullPath, 0755)
	require.NoError(t, err)

	// Create metadata
	metadata := model.BackupMetadata{
		Created:             time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		From:                time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		Namespace:           "test-ns",
		RecordCount:         100,
		ByteCount:           1024,
		FileCount:           1,
		SecondaryIndexCount: 0,
		UDFCount:            0,
	}

	// Write metadata
	err = service.WriteBackupMetadata(ctx, routineName, backupPath, metadata)
	require.NoError(t, err)

	// Verify file exists
	metadataFilePath := filepath.Join(fullPath, metadataFile)
	_, err = os.Stat(metadataFilePath)
	require.NoError(t, err)

	// Get backups
	filter := NewFullBackupFilter(routineName)
	backups, err := service.GetBackups(ctx, filter)

	// Verify
	require.NoError(t, err)
	require.Len(t, backups, 1)

	// Verify backup details
	assert.Equal(t, uint64(100), backups[0].RecordCount)
	assert.Equal(t, uint64(1024), backups[0].ByteCount)
	assert.Equal(t, "test-ns", backups[0].Namespace)
	assert.Equal(t, time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), backups[0].Created)
}

func TestLocalWriteAndGetMultipleBackups(t *testing.T) {
	// Setup
	service, _, tempDir := setupLocalBackupBackendService(t)

	ctx := context.Background()
	routineName := "test-routine"

	// Create first backup
	backupPath1 := filepath.Join(routineName, "backup", "1609459200000", "data", "test-ns")
	fullPath1 := filepath.Join(tempDir, backupPath1)
	err := os.MkdirAll(fullPath1, 0755)
	require.NoError(t, err)

	metadata1 := model.BackupMetadata{
		Created:             time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		From:                time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		Namespace:           "test-ns",
		RecordCount:         100,
		ByteCount:           1024,
		FileCount:           1,
		SecondaryIndexCount: 0,
		UDFCount:            0,
	}

	err = service.WriteBackupMetadata(ctx, routineName, backupPath1, metadata1)
	require.NoError(t, err)

	// Create second backup
	backupPath2 := filepath.Join(routineName, "backup", "1609545600000", "data", "test-ns")
	fullPath2 := filepath.Join(tempDir, backupPath2)
	err = os.MkdirAll(fullPath2, 0755)
	require.NoError(t, err)

	metadata2 := model.BackupMetadata{
		Created:             time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC),
		From:                time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC),
		Namespace:           "test-ns",
		RecordCount:         200,
		ByteCount:           2048,
		FileCount:           2,
		SecondaryIndexCount: 0,
		UDFCount:            0,
	}

	err = service.WriteBackupMetadata(ctx, routineName, backupPath2, metadata2)
	require.NoError(t, err)

	// Get all backups
	filter := NewFullBackupFilter(routineName)
	backups, err := service.GetBackups(ctx, filter)

	// Verify
	require.NoError(t, err)
	require.Len(t, backups, 2)

	// Verify backups are sorted by timestamp
	assert.Equal(t, uint64(100), backups[0].RecordCount)
	assert.Equal(t, time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), backups[0].Created)

	assert.Equal(t, uint64(200), backups[1].RecordCount)
	assert.Equal(t, time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC), backups[1].Created)

	// Test with Last() filter
	lastFilter := NewFullBackupFilter(routineName).Last()
	lastBackups, err := service.GetBackups(ctx, lastFilter)

	// Verify
	require.NoError(t, err)
	require.Len(t, lastBackups, 1)
	assert.Equal(t, uint64(200), lastBackups[0].RecordCount) // Should return the latest backup
}

func TestLocalDeleteBackup(t *testing.T) {
	// Setup
	service, _, _ := setupLocalBackupBackendService(t)

	ctx := context.Background()
	routineName := "test-routine"

	// Create backup
	created := time.UnixMilli(1609459200000)
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
