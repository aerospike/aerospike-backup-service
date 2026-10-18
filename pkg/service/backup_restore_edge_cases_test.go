package service

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/optional"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
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

// --- 7. Restore-by-time boundaries ----------------------------------------------------

// A full backup that started before T but finished after T must not be selected;
// a backup finished exactly at T must be selected (inclusive upper bound).
func TestRestoreByTime_FinishedBoundaryIsInclusive(t *testing.T) {
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

	runner := &timeRestoreRunner{backupReader: catalog}

	// T inside the second full's run window -> first full is chosen.
	chains, err := runner.findBackupsToRestore(t.Context(), &model.RestoreTimestampRequest{
		RoutineName: "r", Storage: st, Time: ts2.Add(5 * time.Minute)})
	require.NoError(t, err)
	assert.Equal(t, ts1, chains["ns"][0].Created)

	// T == Finished of the second full -> second full is chosen (inclusive).
	chains, err = runner.findBackupsToRestore(t.Context(), &model.RestoreTimestampRequest{
		RoutineName: "r", Storage: st, Time: ts2.Add(10 * time.Minute)})
	require.NoError(t, err)
	assert.Equal(t, ts2, chains["ns"][0].Created)
}

// filterIncrementals: differential chain keeps every incremental, cumulative chain
// keeps only the newest one; incrementals not strictly after the full are dropped.
func TestRestoreByTime_IncrementalChainSelection(t *testing.T) {
	full := model.BackupDetails{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(1000)}}
	mk := func(created, from int64) model.BackupDetails {
		return model.BackupDetails{BackupMetadata: model.BackupMetadata{
			Created: time.UnixMilli(created), From: time.UnixMilli(from)}}
	}

	// differential
	got := filterIncrementals(full, []model.BackupDetails{mk(3000, 2000), mk(2000, 1000), mk(900, 500)})
	require.Len(t, got, 2)
	assert.Equal(t, int64(2000), got[0].Created.UnixMilli())
	assert.Equal(t, int64(3000), got[1].Created.UnixMilli())

	// cumulative
	got = filterIncrementals(full, []model.BackupDetails{mk(3000, 1000), mk(2000, 1000)})
	require.Len(t, got, 1)
	assert.Equal(t, int64(3000), got[0].Created.UnixMilli())

	// incremental with Created == full.Created is not part of the chain
	got = filterIncrementals(full, []model.BackupDetails{mk(1000, 500)})
	assert.Empty(t, got)
}

// --- 8. Restore-by-time "empty namespace" optimisation checks the SOURCE namespace ---

// prepareNamespaceRestore reverses the chain and switches to CREATE_ONLY when the target
// namespace is empty; with a namespace remap the record count has to come from the destination
// namespace, not the source one.
func TestRestoreByTime_EmptyCheckUsesDestinationNamespace(t *testing.T) {
	ctrl := gomock.NewController(t)
	info := aerospike.NewMockInfoGetter(ctrl)

	var queried string
	info.EXPECT().GetRecordCount(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, ns string, _ []string) (uint64, error) {
			queried = ns
			return 0, nil
		})

	req := &model.RestoreTimestampRequest{
		Policy: model.RestorePolicy{
			Namespace: &model.RestoreNamespace{Source: "source-ns", Destination: "dest-ns"},
		},
	}
	backups := []model.BackupDetails{{BackupMetadata: model.BackupMetadata{Namespace: "source-ns"}}}

	policy, err := prepareNamespaceRestore(t.Context(), info, "source-ns", req, backups, slog.Default())
	require.NoError(t, err)
	assert.Equal(t, "dest-ns", queried, "emptiness must be checked on the namespace the data is written to")
	assert.True(t, *policy.Unique, "empty destination switches to CREATE_ONLY")
}
