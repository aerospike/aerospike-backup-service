package serverbackup

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/optional"
	listermodels "github.com/aerospike/backup-go/pkg/server/lister/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// retentionFull returns a full backup of the namespace with the key, created minutes after
// catalogStart.
func retentionFull(key, namespace string, minutes int) model.BackupDetails {
	return model.BackupDetails{
		BackupMetadata: model.BackupMetadata{Namespace: namespace, Created: catalogAt(minutes)},
		Key:            key,
	}
}

// retentionIncr returns an incremental backup of the namespace with the key, created minutes after
// catalogStart.
func retentionIncr(key, namespace string, minutes int) model.BackupDetails {
	backup := retentionFull(key, namespace, minutes)
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

// TestRetentionManager_ApplyRetention checks that retention deletes what the rule expires, here
// for a routine that no longer backs up baz, holds the routine while it lists and deletes, and
// releases it afterwards.
func TestRetentionManager_ApplyRetention(t *testing.T) {
	t.Parallel()
	catalog := NewMockCatalog(gomock.NewController(t))
	routine := retentionRoutine(&model.RetentionPolicy{FullBackups: optional.Of(1)})
	routine.Namespaces = []string{"bar"}
	routineStorage := &collections.LockMap{}
	lock := routineStorage.Get("routine")
	held := func() {
		if lock.TryRLock() {
			lock.RUnlock()
			t.Error("a restore could start while retention works")
		}
	}

	catalog.EXPECT().AllBackups(t.Context(), routine).DoAndReturn(
		func(context.Context, *model.BackupRoutine) ([]model.BackupDetails, error) {
			held()

			return []model.BackupDetails{
				retentionFull("baz1", "baz", 0),
				retentionIncr("bazi", "baz", 2),
				retentionFull("baz2", "baz", 5), // baz keeps its own last full backup
				retentionFull("bar1", "bar", 10),
				retentionFull("bar2", "bar", 20),
			}, nil
		})
	for _, key := range []string{"baz1", "bazi", "bar1"} {
		catalog.EXPECT().Delete(t.Context(), routine.Storage, key).DoAndReturn(
			func(context.Context, model.Storage, string) error {
				held()

				return nil
			})
	}

	require.NoError(t, NewRetentionManager(catalog, routineStorage).ApplyRetention(t.Context(), routine))
	assert.True(t, lock.TryLock(), "the routine stayed locked after the cleanup")
}

// TestRetentionManager_OverTheCatalog runs retention over the real catalog: from the listed
// metadata to the folders handed to the storage.
func TestRetentionManager_OverTheCatalog(t *testing.T) {
	t.Parallel()

	const (
		oldFull = "261028T120000-ful1"
		oldIncr = "261028T121000-inc1"
		aborted = "261028T121500-abrt"
		newFull = "261028T122000-ful2"
		bazFull = "261028T120000-baz1"
	)

	source := &fakeSource{metadata: []listermodels.Metadata{
		fullMetadata(oldFull, "bar", catalogAt(0)),
		incrMetadata(oldIncr, "bar", catalogAt(0), catalogAt(10)),
		{BackupID: aborted, Namespace: "bar", Status: listermodels.BackupStatusAborted},
		fullMetadata(newFull, "bar", catalogAt(20)),
		fullMetadata(bazFull, "baz", catalogAt(0)),
	}}
	operations := storage.NewMockOperations(gomock.NewController(t))
	routine := retentionRoutine(&model.RetentionPolicy{FullBackups: optional.Of(1)})

	operations.EXPECT().DeleteFolder(t.Context(), routine.Storage, "routine/"+oldFull).Return(nil)
	operations.EXPECT().DeleteFolder(t.Context(), routine.Storage, "routine/"+oldIncr).Return(nil)

	manager := NewRetentionManager(&catalog{sources: source, operations: operations}, &collections.LockMap{})

	require.NoError(t, manager.ApplyRetention(t.Context(), routine))
	assert.Empty(t, source.statsRead)
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
	routineStorage := &collections.LockMap{}

	catalog.EXPECT().AllBackups(t.Context(), routine).Return([]model.BackupDetails{
		retentionFull("ful1", "bar", 0),
		retentionFull("ful2", "bar", 20),
		retentionFull("ful3", "bar", 40),
	}, nil)
	catalog.EXPECT().Delete(t.Context(), routine.Storage, "ful1").Return(errors.New("denied"))
	catalog.EXPECT().Delete(t.Context(), routine.Storage, "ful2").Return(nil)

	err := NewRetentionManager(catalog, routineStorage).ApplyRetention(t.Context(), routine)

	require.EqualError(t, err, "failed to delete server backup ful1: denied")
	assert.True(t, routineStorage.Get("routine").TryLock(), "the routine stayed locked after the cleanup")
}

// TestRetentionManager_ListFails checks that a listing that fails deletes nothing.
func TestRetentionManager_ListFails(t *testing.T) {
	t.Parallel()
	catalog := NewMockCatalog(gomock.NewController(t))
	routine := retentionRoutine(&model.RetentionPolicy{FullBackups: optional.Of(1)})
	routineStorage := &collections.LockMap{}

	catalog.EXPECT().AllBackups(t.Context(), routine).Return(nil, errors.New("unreachable"))

	err := NewRetentionManager(catalog, routineStorage).ApplyRetention(t.Context(), routine)

	require.EqualError(t, err, "failed to list the backups of routine routine: unreachable")
	assert.True(t, routineStorage.Get("routine").TryLock(), "the routine stayed locked after the cleanup")
}

func TestExpiredBackups(t *testing.T) {
	t.Parallel()

	chain := []model.BackupDetails{
		retentionFull("ful1", "bar", 0),
		retentionIncr("inc1", "bar", 10),
		retentionFull("ful2", "bar", 20),
		retentionIncr("inc2", "bar", 30),
		retentionFull("ful3", "bar", 40),
		retentionIncr("inc3", "bar", 50),
	}
	newestFirst := slices.Clone(chain)
	slices.Reverse(newestFirst)

	tests := []struct {
		name    string
		policy  *model.RetentionPolicy
		backups []model.BackupDetails
		want    []string
	}{
		{
			name:    "full backups beyond the count, with their incremental backups",
			policy:  &model.RetentionPolicy{FullBackups: optional.Of(2)},
			backups: chain,
			want:    []string{"ful1", "inc1"},
		},
		{
			name:    "incremental backups of fewer full backups",
			policy:  &model.RetentionPolicy{FullBackups: optional.Of(2), IncrBackups: optional.Of(1)},
			backups: chain,
			want:    []string{"ful1", "inc1", "inc2"},
		},
		{
			name:    "incremental count alone keeps every full backup",
			policy:  &model.RetentionPolicy{IncrBackups: optional.Of(1)},
			backups: chain,
			want:    []string{"inc1", "inc2"},
		},
		{
			name:    "no incremental backup",
			policy:  &model.RetentionPolicy{IncrBackups: optional.Of(0)},
			backups: chain,
			want:    []string{"inc1", "inc2", "inc3"},
		},
		{
			name:    "no more full backups than the count",
			policy:  &model.RetentionPolicy{FullBackups: optional.Of(3)},
			backups: chain,
			want:    []string{},
		},
		{
			name:    "a listing that is not oldest first",
			policy:  &model.RetentionPolicy{FullBackups: optional.Of(1)},
			backups: newestFirst,
			want:    []string{"inc2", "ful2", "inc1", "ful1"},
		},
		{
			name:   "each namespace on its own",
			policy: &model.RetentionPolicy{FullBackups: optional.Of(1)},
			backups: []model.BackupDetails{
				retentionFull("bar1", "bar", 0),
				retentionFull("baz1", "baz", 1), // the same run, started later
				retentionIncr("bazi", "baz", 10),
				retentionFull("bar2", "bar", 20),
				// baz failed in the second run.
			},
			want: []string{"bar1"},
		},
		{
			name:   "an incremental backup as old as the oldest kept full one",
			policy: &model.RetentionPolicy{FullBackups: optional.Of(1)},
			backups: []model.BackupDetails{
				retentionFull("ful1", "bar", 0),
				retentionIncr("inc1", "bar", 20),
				retentionFull("ful2", "bar", 20),
			},
			want: []string{"ful1", "inc1"},
		},
		{
			name:   "exactly as many full backups as the count",
			policy: &model.RetentionPolicy{FullBackups: optional.Of(2)},
			backups: []model.BackupDetails{
				retentionFull("ful1", "bar", 0),
				retentionIncr("inc0", "bar", 0),
				retentionFull("ful2", "bar", 20),
			},
			want: []string{},
		},
		{
			name:   "full backups created at the same time are kept together",
			policy: &model.RetentionPolicy{FullBackups: optional.Of(1)},
			backups: []model.BackupDetails{
				retentionFull("ful1", "bar", 0),
				retentionFull("ful2", "bar", 20),
				retentionFull("ful3", "bar", 20),
			},
			want: []string{"ful1"},
		},
		{
			name:   "full backups created at the same time count once",
			policy: &model.RetentionPolicy{FullBackups: optional.Of(2)},
			backups: []model.BackupDetails{
				retentionFull("ful1", "bar", 0),
				retentionFull("ful2", "bar", 20),
				retentionFull("ful3", "bar", 20),
			},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			before := slices.Clone(tt.backups)

			assert.Equal(t, tt.want, keys(expiredBackups(tt.policy, tt.backups)))
			assert.Equal(t, before, tt.backups, "the listing was changed")
		})
	}
}
