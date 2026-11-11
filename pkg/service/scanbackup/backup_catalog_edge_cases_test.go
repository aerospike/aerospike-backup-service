package scanbackup

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/optional"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testLockMap collections.LockMap

func newLocalTestCatalog(t *testing.T) (BackupCatalog, *model.LocalStorage, string) {
	t.Helper()
	dir := t.TempDir()
	st := &model.LocalStorage{Path: dir}
	ops := storage.NewOperations(storage.NewLocalStorageAccessor())
	return NewBackupCatalog(NewPathService(nil), ops), st, dir
}

func writeTestMetadata(t *testing.T, root, routine string, bt model.BackupType, ts time.Time, ns string,
	from time.Time, fileCount uint64) {
	t.Helper()
	ps := NewPathService(nil)
	folder := filepath.Join(root, ps.GetBackupPath(routine, bt, ns, ts))
	require.NoError(t, os.MkdirAll(folder, 0o750))
	md := model.BackupMetadata{
		Created:   ts,
		Finished:  ts.Add(time.Second),
		From:      from,
		Namespace: ns,
		FileCount: fileCount,
	}
	catalog := NewBackupCatalog(ps, storage.NewOperations(storage.NewLocalStorageAccessor()))
	r := &model.BackupRoutine{Name: routine, Storage: &model.LocalStorage{Path: root}}
	require.NoError(t, catalog.WriteBackupMetadata(t.Context(), r, ps.GetBackupPath(routine, bt, ns, ts), md))
}

// deleteFullBackups issues one Delete per namespace metadata for the same timestamp folder;
// the second delete of an already-removed folder does not fail the retention run (local storage).
func TestRetention_DuplicateTimestampDeleteIsIdempotentOnLocal(t *testing.T) {
	catalog, st, root := newLocalTestCatalog(t)
	routine := &model.BackupRoutine{
		Name: "r", Storage: st,
		BackupPolicy: &model.BackupPolicy{
			RetentionPolicy: &model.RetentionPolicy{FullBackups: optional.Of(1)},
		},
	}
	ts1 := time.UnixMilli(1_700_000_000_000).UTC()
	ts2 := ts1.Add(time.Hour)
	writeTestMetadata(t, root, "r", model.BackupTypeFull, ts1, "ns1", time.Time{}, 1)
	writeTestMetadata(t, root, "r", model.BackupTypeFull, ts1, "ns2", time.Time{}, 1)
	writeTestMetadata(t, root, "r", model.BackupTypeFull, ts2, "ns1", time.Time{}, 1)
	writeTestMetadata(t, root, "r", model.BackupTypeFull, ts2, "ns2", time.Time{}, 1)

	rm := NewBackupRetentionManager(catalog, &testLockMap)
	require.NoError(t, rm.ApplyRetention(t.Context(), routine))

	_, err := os.Stat(filepath.Join(root, "r", "backup", "1700000000000"))
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(root, "r", "backup", "1700003600000"))
	assert.NoError(t, err)
}

// A full backup that started before T but finished after T is not listed up to T. A backup that
// finished exactly at T is listed: the upper bound is inclusive.
func TestGetBackups_FinishedBoundaryIsInclusive(t *testing.T) {
	catalog, st, root := newLocalTestCatalog(t)
	ps := NewPathService(nil)
	ts1 := time.UnixMilli(1_700_000_000_000).UTC()
	ts2 := ts1.Add(time.Hour)
	r := &model.BackupRoutine{Name: "r", Storage: st}

	write := func(ts, finished time.Time) {
		p := ps.GetBackupPath("r", model.BackupTypeFull, "ns", ts)
		require.NoError(t, os.MkdirAll(filepath.Join(root, p), 0o750))
		require.NoError(t, catalog.WriteBackupMetadata(t.Context(), r, p, model.BackupMetadata{
			Created: ts, Finished: finished, Namespace: "ns", FileCount: 1,
		}))
	}
	write(ts1, ts1.Add(time.Minute))
	write(ts2, ts2.Add(10*time.Minute)) // long-running full

	created := func(upTo time.Time) []time.Time {
		backups, err := catalog.GetBackups(t.Context(), service.NewFullBackupFilter(r).WithToTime(upTo))
		require.NoError(t, err)

		times := make([]time.Time, 0, len(backups))
		for _, b := range backups {
			times = append(times, b.Created)
		}

		return times
	}

	// T inside the second full's run window: only the first full is listed.
	assert.Equal(t, []time.Time{ts1}, created(ts2.Add(5*time.Minute)))
	// T == Finished of the second full: both are listed.
	assert.Equal(t, []time.Time{ts1, ts2}, created(ts2.Add(10*time.Minute)))

	// A restore to T therefore starts from the latest full backup that had finished by T.
	restoresFrom := func(upTo time.Time) time.Time {
		chains, err := service.FindRestoreChains(t.Context(), catalog,
			&model.RestoreTimestampRequest{RoutineName: "r", Storage: st, Time: upTo})
		require.NoError(t, err)

		return chains["ns"].Full.Created
	}

	assert.Equal(t, ts1, restoresFrom(ts2.Add(5*time.Minute)))
	assert.Equal(t, ts2, restoresFrom(ts2.Add(10*time.Minute)))
}
