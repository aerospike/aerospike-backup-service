package serverbackup

import (
	"errors"
	"slices"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/optional"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// retainedKey returns the key of the routine's backup whose job id ends with salt, four base36
// digits that name the backup in a test.
func retainedKey(salt string) string {
	return backupKey("routine", "261028T120000-"+salt)
}

// retainedFull returns a full backup of the namespace, created minutes after catalogStart.
func retainedFull(salt, namespace string, minutes int) model.BackupDetails {
	return model.BackupDetails{
		BackupMetadata: model.BackupMetadata{Namespace: namespace, Created: catalogAt(minutes)},
		Key:            retainedKey(salt),
	}
}

// retainedIncr returns an incremental backup of the namespace, created minutes after catalogStart.
func retainedIncr(salt, namespace string, minutes int) model.BackupDetails {
	backup := retainedFull(salt, namespace, minutes)
	backup.From = catalogAt(minutes - 5)

	return backup
}

func retentionRoutine(policy *model.RetentionPolicy) *model.BackupRoutine {
	return &model.BackupRoutine{
		Name:         "routine",
		Storage:      &model.S3Storage{},
		BackupPolicy: &model.BackupPolicy{RetentionPolicy: policy},
	}
}

func TestRetentionManager_ApplyRetention(t *testing.T) {
	t.Parallel()

	// Three full backups of bar, each followed by an incremental one.
	chain := []model.BackupDetails{
		retainedFull("ful1", "bar", 0),
		retainedIncr("inc1", "bar", 10),
		retainedFull("ful2", "bar", 20),
		retainedIncr("inc2", "bar", 30),
		retainedFull("ful3", "bar", 40),
		retainedIncr("inc3", "bar", 50),
	}

	tests := []struct {
		name        string
		policy      *model.RetentionPolicy
		backups     []model.BackupDetails
		wantDeleted []string
	}{
		{
			name:        "full backups beyond the count, with their incremental backups",
			policy:      &model.RetentionPolicy{FullBackups: optional.Of(2)},
			backups:     chain,
			wantDeleted: []string{"ful1", "inc1"},
		},
		{
			name:        "incremental backups of fewer full backups",
			policy:      &model.RetentionPolicy{FullBackups: optional.Of(2), IncrBackups: optional.Of(1)},
			backups:     chain,
			wantDeleted: []string{"ful1", "inc1", "inc2"},
		},
		{
			name:        "incremental count alone keeps every full backup",
			policy:      &model.RetentionPolicy{IncrBackups: optional.Of(1)},
			backups:     chain,
			wantDeleted: []string{"inc1", "inc2"},
		},
		{
			name:        "no incremental backup",
			policy:      &model.RetentionPolicy{IncrBackups: optional.Of(0)},
			backups:     chain,
			wantDeleted: []string{"inc1", "inc2", "inc3"},
		},
		{
			name:    "no more full backups than the count",
			policy:  &model.RetentionPolicy{FullBackups: optional.Of(3)},
			backups: chain,
		},
		{
			name:   "each namespace on its own",
			policy: &model.RetentionPolicy{FullBackups: optional.Of(1)},
			backups: []model.BackupDetails{
				retainedFull("bar1", "bar", 0),
				retainedFull("baz1", "baz", 1), // the same run, started later
				retainedIncr("bazi", "baz", 10),
				retainedFull("bar2", "bar", 20),
				// baz failed in the second run.
			},
			wantDeleted: []string{"bar1"},
		},
		{
			name:   "an incremental backup as old as the oldest kept full one",
			policy: &model.RetentionPolicy{FullBackups: optional.Of(1)},
			backups: []model.BackupDetails{
				retainedFull("ful1", "bar", 0),
				retainedIncr("inc1", "bar", 20),
				retainedFull("ful2", "bar", 20),
			},
			wantDeleted: []string{"ful1", "inc1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			catalog := NewMockCatalog(gomock.NewController(t))
			routine := retentionRoutine(tt.policy)

			// The manager filters the listing in place, and the cases share one.
			catalog.EXPECT().AllBackups(t.Context(), routine).Return(slices.Clone(tt.backups), nil)
			for _, salt := range tt.wantDeleted {
				catalog.EXPECT().Delete(t.Context(), routine, retainedKey(salt)).Return(nil)
			}

			err := NewRetentionManager(catalog, &collections.LockMap{}).ApplyRetention(t.Context(), routine)

			require.NoError(t, err)
		})
	}
}

// TestRetentionManager_NoPolicy checks that a routine without a retention count is not even listed.
func TestRetentionManager_NoPolicy(t *testing.T) {
	t.Parallel()

	for _, policy := range []*model.RetentionPolicy{nil, {}} {
		manager := NewRetentionManager(NewMockCatalog(gomock.NewController(t)), &collections.LockMap{})

		require.NoError(t, manager.ApplyRetention(t.Context(), retentionRoutine(policy)))
	}
}

// TestRetentionManager_RestoreRunning checks that nothing is deleted while a restore holds the
// routine's lock.
func TestRetentionManager_RestoreRunning(t *testing.T) {
	t.Parallel()

	routineStorage := &collections.LockMap{}
	manager := NewRetentionManager(NewMockCatalog(gomock.NewController(t)), routineStorage)

	restore := routineStorage.Get("routine")
	restore.RLock()
	defer restore.RUnlock()

	require.NoError(t, manager.ApplyRetention(t.Context(),
		retentionRoutine(&model.RetentionPolicy{FullBackups: optional.Of(1)})))
}

// TestRetentionManager_DeleteFails checks that a backup that cannot be deleted is reported and
// does not keep the others from being deleted.
func TestRetentionManager_DeleteFails(t *testing.T) {
	t.Parallel()
	catalog := NewMockCatalog(gomock.NewController(t))
	routine := retentionRoutine(&model.RetentionPolicy{FullBackups: optional.Of(1)})

	catalog.EXPECT().AllBackups(t.Context(), routine).Return([]model.BackupDetails{
		retainedFull("ful1", "bar", 0),
		retainedFull("ful2", "bar", 20),
		retainedFull("ful3", "bar", 40),
	}, nil)
	catalog.EXPECT().Delete(t.Context(), routine, retainedKey("ful1")).Return(errors.New("denied"))
	catalog.EXPECT().Delete(t.Context(), routine, retainedKey("ful2")).Return(nil)

	err := NewRetentionManager(catalog, &collections.LockMap{}).ApplyRetention(t.Context(), routine)

	require.EqualError(t, err, "failed to delete server backup "+retainedKey("ful1")+": denied")
}

// TestRetentionManager_ListFails checks that a listing that fails deletes nothing.
func TestRetentionManager_ListFails(t *testing.T) {
	t.Parallel()
	catalog := NewMockCatalog(gomock.NewController(t))
	routine := retentionRoutine(&model.RetentionPolicy{FullBackups: optional.Of(1)})

	catalog.EXPECT().AllBackups(t.Context(), routine).Return(nil, errors.New("unreachable"))

	err := NewRetentionManager(catalog, &collections.LockMap{}).ApplyRetention(t.Context(), routine)

	require.EqualError(t, err, "failed to list the backups of routine routine: unreachable")
}

// TestExpiredBackups_SameCreationTime checks that full backups created in the same second are
// kept together.
func TestExpiredBackups_SameCreationTime(t *testing.T) {
	t.Parallel()

	expired := expiredBackups(&model.RetentionPolicy{FullBackups: optional.Of(1)}, []model.BackupDetails{
		retainedFull("ful1", "bar", 0),
		retainedFull("ful2", "bar", 20),
		retainedFull("ful3", "bar", 20),
	})

	require.Equal(t, []string{retainedKey("ful1")}, keys(expired))
}
