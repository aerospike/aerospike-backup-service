package service

import (
	"context"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// TestRetentionManagerImpl_deleteOldBackups_full tests the case where only full backups are deleted.
func TestRetentionManagerImpl_deleteOldBackups_full(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	routineName := "testRoutine"
	retentionPolicy := model.RetentionPolicy{
		FullBackups: util.Ptr(2),
	}

	backendService := NewMockBackupReaderWriter(ctrl)

	config := model.NewConfig()
	testRoutine := &model.BackupRoutine{
		BackupPolicy: &model.BackupPolicy{
			RetentionPolicy: &retentionPolicy,
		},
	}
	_ = config.AddRoutine(routineName, testRoutine)

	retentionManager := NewBackupRetentionManager(backendService, config)

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
	backendService.EXPECT().Delete(ctx, routineName, "testRoutine/backup/1000").Return(nil)
	backendService.EXPECT().Delete(ctx, routineName, "testRoutine/backup/2000").Return(nil)
	backendService.EXPECT().Delete(ctx, routineName, "testRoutine/backup/3000").Return(nil)

	// Expect calls to get incrementals.
	backendService.EXPECT().GetBackups(ctx,
		NewIncrementalBackupFilter(routineName).WithToTime(time.UnixMilli(4000))).
		Return([]model.BackupDetails{}, nil)

	err := retentionManager.deleteOldBackups(ctx, routineName)
	assert.NoError(t, err)
}

// TestRetentionManagerImpl_deleteOldBackups_fullWithIncrementals tests that when a full backup is deleted,
// its corresponding incremental backups are also deleted.
func TestRetentionManagerImpl_deleteOldBackups_fullWithIncrementals(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	routineName := "testRoutine"
	retentionPolicy := model.RetentionPolicy{
		FullBackups: util.Ptr(1),
	}

	backendService := NewMockBackupReaderWriter(ctrl)

	config := model.NewConfig()
	testRoutine := &model.BackupRoutine{
		BackupPolicy: &model.BackupPolicy{
			RetentionPolicy: &retentionPolicy,
		},
	}
	_ = config.AddRoutine(routineName, testRoutine)

	retentionManager := NewBackupRetentionManager(backendService, config)

	fullBackups := []model.BackupDetails{
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(1000)}}, // to be deleted
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(2000)}}, // keep
	}
	backendService.EXPECT().GetBackups(ctx, NewFullBackupFilter(routineName)).
		Return(fullBackups, nil)

	// Expect deletion of the first full backup
	backendService.EXPECT().Delete(ctx, routineName, "testRoutine/backup/1000").Return(nil)

	// Expect a call to get incrementals for the deleted full backup
	incrementals := []model.BackupDetails{
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(1100)}},
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(1200)}},
	}
	backendService.EXPECT().
		GetBackups(ctx, NewIncrementalBackupFilter(routineName).WithToTime(time.UnixMilli(2000))).
		Return(incrementals, nil)

	// Expect deletion of the incrementals
	backendService.EXPECT().Delete(ctx, routineName, "testRoutine/incremental/1100").Return(nil)
	backendService.EXPECT().Delete(ctx, routineName, "testRoutine/incremental/1200").Return(nil)

	err := retentionManager.deleteOldBackups(ctx, routineName)
	assert.NoError(t, err)
}

// TestRetentionManagerImpl_deleteOldBackups_incrementalPolicy tests the incremental retention policy.
func TestRetentionManagerImpl_deleteOldBackups_incrementalPolicy(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	routineName := "testRoutine"
	retentionPolicy := model.RetentionPolicy{
		FullBackups: util.Ptr(2),
		IncrBackups: util.Ptr(1),
	}

	backendService := NewMockBackupReaderWriter(ctrl)

	config := model.NewConfig()
	testRoutine := &model.BackupRoutine{
		BackupPolicy: &model.BackupPolicy{
			RetentionPolicy: &retentionPolicy,
		},
	}
	_ = config.AddRoutine(routineName, testRoutine)

	retentionManager := NewBackupRetentionManager(backendService, config)

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
	backendService.EXPECT().Delete(ctx, routineName, "testRoutine/incremental/1100").Return(nil)
	backendService.EXPECT().Delete(ctx, routineName, "testRoutine/incremental/1200").Return(nil)

	err := retentionManager.deleteOldBackups(ctx, routineName)
	assert.NoError(t, err)
}

// TestRetentionManagerImpl_deleteOldBackups_noPolicy tests the case where no retention policy is defined.
func TestRetentionManagerImpl_deleteOldBackups_noPolicy(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	routineName := "testRoutine"

	backendService := NewMockBackupReaderWriter(ctrl) // Expects no calls

	config := model.NewConfig()
	testRoutine := &model.BackupRoutine{
		BackupPolicy: &model.BackupPolicy{
			RetentionPolicy: nil, // No retention policy
		},
	}
	_ = config.AddRoutine(routineName, testRoutine)

	retentionManager := NewBackupRetentionManager(backendService, config)

	err := retentionManager.deleteOldBackups(ctx, routineName)
	assert.NoError(t, err)
}

// TestRetentionManagerImpl_deleteOldBackups_noneToDelete tests the case where the number of backups
// is less than or equal to the retention count.
func TestRetentionManagerImpl_deleteOldBackups_noneToDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	routineName := "testRoutine"
	retentionPolicy := model.RetentionPolicy{
		FullBackups: util.Ptr(5),
	}

	backendService := NewMockBackupReaderWriter(ctrl)

	config := model.NewConfig()
	testRoutine := &model.BackupRoutine{
		BackupPolicy: &model.BackupPolicy{
			RetentionPolicy: &retentionPolicy,
		},
	}
	_ = config.AddRoutine(routineName, testRoutine)

	retentionManager := NewBackupRetentionManager(backendService, config)

	fullBackups := []model.BackupDetails{
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(1000)}},
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(2000)}},
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(3000)}},
	}
	backendService.EXPECT().GetBackups(ctx, NewFullBackupFilter(routineName)).
		Return(fullBackups, nil)

	// No Delete calls are expected

	err := retentionManager.deleteOldBackups(ctx, routineName)
	assert.NoError(t, err)
}

// TestRetentionManagerImpl_deleteOldBackups_retainZeroIncrementals tests the case where the retention
// count for incremental backups is 0.
func TestRetentionManagerImpl_deleteOldBackups_retainZeroIncrementals(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	routineName := "testRoutine"
	retentionPolicy := model.RetentionPolicy{
		IncrBackups: util.Ptr(0),
	}

	backendService := NewMockBackupReaderWriter(ctrl)
	config := model.NewConfig()
	testRoutine := &model.BackupRoutine{
		BackupPolicy: &model.BackupPolicy{
			RetentionPolicy: &retentionPolicy,
		},
	}
	_ = config.AddRoutine(routineName, testRoutine)

	retentionManager := NewBackupRetentionManager(backendService, config)

	// GetBackups is still called for full backups
	backendService.EXPECT().GetBackups(ctx, NewFullBackupFilter(routineName)).
		Return([]model.BackupDetails{}, nil)

	// Expect a single delete call for the incremental root path
	backendService.EXPECT().Delete(ctx, routineName, "testRoutine/incremental").Return(nil)

	err := retentionManager.deleteOldBackups(ctx, routineName)
	assert.NoError(t, err)
}

// TestRetentionManagerImpl_deleteOldBackups_concurrencyLock tests that the concurrency lock
// prevents multiple retention jobs from running for the same routine.
func TestRetentionManagerImpl_deleteOldBackups_concurrencyLock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	routineName := "testRoutine"
	retentionPolicy := model.RetentionPolicy{
		FullBackups: util.Ptr(1),
	}

	backendService := NewMockBackupReaderWriter(ctrl) // Expects no calls

	config := model.NewConfig()
	testRoutine := &model.BackupRoutine{
		BackupPolicy: &model.BackupPolicy{
			RetentionPolicy: &retentionPolicy,
		},
	}
	_ = config.AddRoutine(routineName, testRoutine)

	// Manually create the manager to access the internal lock
	retentionManager := &RetentionManagerImpl{
		backendService: backendService,
		config:         config,
	}

	// Simulate lock being held by another process
	mu := retentionManager.locks.Get(routineName)
	mu.Lock()
	defer mu.Unlock()

	// This call should be skipped due to the lock
	err := retentionManager.deleteOldBackups(ctx, routineName)
	assert.NoError(t, err)
}
