package service

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// TestRetentionManager_FullBackupsOnly
// tests that only full backups are deleted when only a full backup retention policy is set.
func TestRetentionManager_FullBackupsOnly(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	backendService := NewMockBackupReaderWriter(ctrl)

	config := configWithRetentionPolicy(&model.RetentionPolicy{
		FullBackups: ptr.Of(2),
	})

	retentionManager := NewBackupRetentionManager(backendService, config, &collections.LockMap{}, NewPathService())

	fullBackups := []model.BackupDetails{
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(1000)}}, // to be deleted
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(2000)}}, // to be deleted
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(3000)}}, // to be deleted
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(4000)}}, // keep
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(5000)}}, // keep
	}
	backendService.EXPECT().GetBackups(ctx, NewFullBackupFilter(routineName)).
		Return(fullBackups, nil)

	// Expect deletion of the first 3 full backups (keep last 2)
	backendService.EXPECT().Delete(ctx, routineName, "test-routine/backup/1000").Return(nil)
	backendService.EXPECT().Delete(ctx, routineName, "test-routine/backup/2000").Return(nil)
	backendService.EXPECT().Delete(ctx, routineName, "test-routine/backup/3000").Return(nil)

	// Expect calls to get incremental backups.
	backendService.EXPECT().GetBackups(ctx,
		NewIncrementalBackupFilter(routineName).WithToTime(time.UnixMilli(4000))).
		Return([]model.BackupDetails{}, nil)

	err := retentionManager.deleteOldBackups(ctx, routineName)
	assert.NoError(t, err)
}

// TestRetentionManager_FullAndIncremental
// tests that when a full backup is deleted, its corresponding incremental backups are also deleted
// (even without incremental retention policy).
func TestRetentionManager_FullAndIncremental(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	backendService := NewMockBackupReaderWriter(ctrl)

	config := configWithRetentionPolicy(&model.RetentionPolicy{
		FullBackups: ptr.Of(1),
	})

	retentionManager := NewBackupRetentionManager(backendService, config, &collections.LockMap{}, NewPathService())

	fullBackups := []model.BackupDetails{
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(1000)}}, // to be deleted
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(2000)}}, // keep
	}
	backendService.EXPECT().GetBackups(ctx, NewFullBackupFilter(routineName)).
		Return(fullBackups, nil)

	// Expect deletion of the first full backup
	backendService.EXPECT().Delete(ctx, routineName, "test-routine/backup/1000").Return(nil)

	// Expect a call to get incrementals for the deleted full backup
	incrementals := []model.BackupDetails{
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(1100)}},
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(1200)}},
	}
	backendService.EXPECT().
		GetBackups(ctx, NewIncrementalBackupFilter(routineName).WithToTime(time.UnixMilli(2000))).
		Return(incrementals, nil)

	// Expect deletion of the incrementals
	backendService.EXPECT().Delete(ctx, routineName, "test-routine/incremental/1100").Return(nil)
	backendService.EXPECT().Delete(ctx, routineName, "test-routine/incremental/1200").Return(nil)

	err := retentionManager.deleteOldBackups(ctx, routineName)
	assert.NoError(t, err)
}

// TestRetentionManager_IncrementalPolicy tests the incremental retention policy.
func TestRetentionManager_IncrementalPolicy(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := configWithRetentionPolicy(&model.RetentionPolicy{
		FullBackups: ptr.Of(2),
		IncrBackups: ptr.Of(1),
	})

	backendService := NewMockBackupReaderWriter(ctrl)

	retentionManager := NewBackupRetentionManager(backendService, config, &collections.LockMap{}, NewPathService())

	fullBackups := []model.BackupDetails{
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(1000)}}, // keep
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(2000)}}, // keep
	}
	backendService.EXPECT().GetBackups(ctx, NewFullBackupFilter(routineName)).
		Return(fullBackups, nil)

	// Expect a call to get incrementals for the retained full backups
	incrementals := []model.BackupDetails{
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(1100)}},
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(1200)}},
	}
	backendService.EXPECT().
		GetBackups(ctx, NewIncrementalBackupFilter(routineName).WithToTime(time.UnixMilli(2000))).
		Return(incrementals, nil)

	// Expect deletion of the older incrementals
	backendService.EXPECT().Delete(ctx, routineName, "test-routine/incremental/1100_01-Jan-1970-02-00-01").Return(nil)
	backendService.EXPECT().Delete(ctx, routineName, "test-routine/incremental/1200_01-Jan-1970-02-00-01").Return(nil)

	err := retentionManager.deleteOldBackups(ctx, routineName)
	assert.NoError(t, err)
}

// TestRetentionManager_NoPolicy tests the case where no retention policy is defined.
func TestRetentionManager_NoPolicy(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	backendService := NewMockBackupReaderWriter(ctrl) // Expects no calls

	config := configWithRetentionPolicy(nil)

	retentionManager := NewBackupRetentionManager(backendService, config, &collections.LockMap{}, NewPathService())

	err := retentionManager.deleteOldBackups(ctx, routineName)
	assert.NoError(t, err)
}

// TestRetentionManager_NoneToDelete tests the case where the number of backups
// is less than or equal to the retention count.
func TestRetentionManager_NoneToDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	backendService := NewMockBackupReaderWriter(ctrl)

	config := configWithRetentionPolicy(&model.RetentionPolicy{
		FullBackups: ptr.Of(5),
	})

	retentionManager := NewBackupRetentionManager(backendService, config, &collections.LockMap{}, NewPathService())

	fullBackups := []model.BackupDetails{
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(1000)}}, // keep
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(2000)}}, // keep
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(3000)}}, // keep
	}
	backendService.EXPECT().GetBackups(ctx, NewFullBackupFilter(routineName)).
		Return(fullBackups, nil)

	// No Delete calls are expected

	err := retentionManager.deleteOldBackups(ctx, routineName)
	assert.NoError(t, err)
}

// TestRetentionManager_RetainZeroIncrementals
// tests the case where the retention count for incremental backups is 0.
func TestRetentionManager_RetainZeroIncrementals(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := configWithRetentionPolicy(&model.RetentionPolicy{
		IncrBackups: ptr.Of(0),
	})

	backendService := NewMockBackupReaderWriter(ctrl)

	retentionManager := NewBackupRetentionManager(backendService, config, &collections.LockMap{}, NewPathService())

	// GetBackups is still called for full backups
	backendService.EXPECT().GetBackups(ctx, NewFullBackupFilter(routineName)).
		Return([]model.BackupDetails{}, nil)

	// Expect a single delete call for the incremental root path
	backendService.EXPECT().Delete(ctx, routineName, "test-routine/incremental").Return(nil)

	err := retentionManager.deleteOldBackups(ctx, routineName)
	assert.NoError(t, err)
}

// TestRetentionManager_ConcurrencyLock
// tests that the concurrency lock prevents multiple retention jobs from running for the same routine.
func TestRetentionManager_ConcurrencyLock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := configWithRetentionPolicy(&model.RetentionPolicy{
		FullBackups: ptr.Of(1),
	})

	backendService := NewMockBackupReaderWriter(ctrl) // Expects no calls

	storage := &collections.LockMap{}
	retentionManager := NewBackupRetentionManager(backendService, config, storage, NewPathService())

	// Simulate lock being held by another process
	mu := storage.Get(routineName)
	mu.Lock()
	defer mu.Unlock()

	// This call should be skipped due to the lock
	err := retentionManager.deleteOldBackups(ctx, routineName)
	assert.NoError(t, err)
}

// TestRetentionManager_PolicyWithNilCounts
// tests the case where the retention policy is defined but both FullBackups and IncrBackups are nil.
func TestRetentionManager_PolicyWithNilCounts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	backendService := NewMockBackupReaderWriter(ctrl) // Expects no calls

	config := configWithRetentionPolicy(&model.RetentionPolicy{})
	retentionManager := NewBackupRetentionManager(backendService, config, &collections.LockMap{}, NewPathService())

	err := retentionManager.deleteOldBackups(ctx, routineName)
	assert.NoError(t, err)
}

func configWithRetentionPolicy(retentionPolicy *model.RetentionPolicy) *model.Config {
	config := model.NewConfig()
	testRoutine := &model.BackupRoutine{
		BackupPolicy: &model.BackupPolicy{
			RetentionPolicy: retentionPolicy,
		},
	}

	_ = config.AddRoutine(routineName, testRoutine)
	return config
}
