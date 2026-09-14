package service

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/backupexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/optional"
	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// --- helpers -------------------------------------------------------------

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

// --- 1. Sealed backup + retry: Created must stay within the scanned bounds ------------

// A sealed backup scans records with last-update < ToTime. When the first attempt fails and the
// retry succeeds, the metadata Created (which the next incremental uses as its FromTime via
// LatestRun()) must not be later than the ToTime the successful attempt scanned with; records
// modified in between would otherwise be in no backup.
func TestSealedRetry_CreatedStaysWithinScannedBounds(t *testing.T) {
	t.Skip("BKRS-408: OnRetry moves StartTime (Created) while the scan keeps the original ToTime")

	ctrl := gomock.NewController(t)
	exec := backupexecutor.NewMockBackup(ctrl)
	failing := backupexecutor.NewMockBackupHandler(ctrl)
	ok := backupexecutor.NewMockBackupHandler(ctrl)
	writer := NewMockBackupWriter(ctrl)

	runner := NewNamespaceBackupRunner(exec, writer, NewPathService(nil))

	// Run start well in the past, so a retry moves Created by a visible amount.
	start := time.Now().Add(-time.Hour).Truncate(time.Millisecond)
	toTime := start
	routine := &model.BackupRoutine{
		Name:          "r",
		SourceCluster: &model.AerospikeCluster{},
		BackupPolicy: &model.BackupPolicy{
			Sealed: func() *bool { b := true; return &b }(),
			RetryPolicy: &model.RetryPolicy{
				MaxRetries:  optional.Of(1),
				BaseTimeout: optional.Of(time.Duration(0)),
			},
		},
	}
	bounds := model.TimeBounds{ToTime: &toTime}

	var capturedBounds []model.TimeBounds
	exec.EXPECT().Run(gomock.Any(), routine, gomock.Any(), "ns", gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ *model.BackupRoutine, tb model.TimeBounds, _, _ string, _ any, _ *slog.Logger,
		) (backupexecutor.BackupHandler, error) {
			capturedBounds = append(capturedBounds, tb)
			return failing, nil
		})
	failing.EXPECT().Wait(gomock.Any()).Return(errors.New("transient"))
	writer.EXPECT().Delete(gomock.Any(), routine, gomock.Any()).Return(nil)

	exec.EXPECT().Run(gomock.Any(), routine, gomock.Any(), "ns", gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ *model.BackupRoutine, tb model.TimeBounds, _, _ string, _ any, _ *slog.Logger,
		) (backupexecutor.BackupHandler, error) {
			capturedBounds = append(capturedBounds, tb)
			return ok, nil
		})
	ok.EXPECT().Wait(gomock.Any()).Return(nil)
	stats := models.NewBackupStats()
	stats.ReadRecords.Add(1)
	ok.EXPECT().GetStats().Return(stats).AnyTimes()

	var md model.BackupMetadata
	writer.EXPECT().WriteBackupMetadata(gomock.Any(), routine, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ *model.BackupRoutine, _ string, m model.BackupMetadata) error {
			md = m
			return nil
		})

	spec := model.BackupRunSpec{Type: model.BackupTypeFull, StartTime: start, TimeBounds: bounds}
	h := runner.Run(t.Context(), routine, "ns", spec, nil, slog.Default())
	require.NoError(t, h.Wait(t.Context()))

	require.Len(t, capturedBounds, 2)
	require.NotNil(t, capturedBounds[1].ToTime)
	assert.False(t, md.Created.After(*capturedBounds[1].ToTime),
		"Created (%s) must not be later than the sealed ToTime of the successful attempt (%s)",
		md.Created, *capturedBounds[1].ToTime)
}

// --- 2. Partially failed full run: the failed namespace keeps its own history -----------

// Storage is the single source of truth for "last backup time". When the full backup of ns2
// fails while ns1 succeeds, the next incremental of ns2 has to start from ns2's last good
// backup; otherwise records modified in between are never captured and restore-by-time builds a
// chain with an uncovered window.
func TestPartialFullRun_KeepsHistoryOfFailedNamespace(t *testing.T) {
	t.Skip("BKRS-406: FindLastRun is routine-level and picks the newest timestamp with any metadata")

	catalog, st, root := newLocalTestCatalog(t)
	routine := &model.BackupRoutine{Name: "r", Storage: st, BackupPolicy: &model.BackupPolicy{}}

	ts1 := time.UnixMilli(1_700_000_000_000).UTC()
	ts2 := ts1.Add(24 * time.Hour)
	ts3 := ts2.Add(time.Hour)

	// Full #1: both namespaces OK. Full #2: ns2 failed and was cleaned up, ns1 succeeded.
	writeTestMetadata(t, root, "r", model.BackupTypeFull, ts1, "ns1", time.Time{}, 1)
	writeTestMetadata(t, root, "r", model.BackupTypeFull, ts1, "ns2", time.Time{}, 1)
	writeTestMetadata(t, root, "r", model.BackupTypeFull, ts2, "ns1", time.Time{}, 1)

	last, err := NewHistoryManager(catalog).FindLastRun(t.Context(), routine)
	require.NoError(t, err)

	registry := NewMockBackupStateRegistry(gomock.NewController(t))
	registry.EXPECT().GetRoutineState(routine).Return(model.RoutineState{LastRunTime: last}).AnyTimes()
	orch := &backupOrchestrator{registry: registry}
	from := orch.backupFromTime(routine, model.BackupTypeIncremental)
	require.NotNil(t, from)

	// The next incremental is written with the FromTime the orchestrator derived.
	writeTestMetadata(t, root, "r", model.BackupTypeIncremental, ts3, "ns1", *from, 1)
	writeTestMetadata(t, root, "r", model.BackupTypeIncremental, ts3, "ns2", *from, 1)

	// Restore-by-time to now: ns2's chain must have no uncovered window.
	runner := &timeRestoreRunner{backupReader: catalog}
	chains, err := runner.findBackupsToRestore(t.Context(), &model.RestoreTimestampRequest{
		RoutineName: "r", Storage: st, Time: ts3.Add(time.Hour),
	})
	require.NoError(t, err)
	require.Len(t, chains["ns2"], 2)
	assert.False(t, chains["ns2"][1].From.After(chains["ns2"][0].Created),
		"ns2 incremental (From %s) must start no later than ns2's last good full (%s)",
		chains["ns2"][1].From, chains["ns2"][0].Created)
}

// --- 3. A corrupt metadata.yaml must not hide the healthy backups of the routine ------

// A truncated or empty metadata.yaml (a write interrupted by a crash or a full disk) must be
// skipped by the listing, so the healthy backups stay visible to history, retention and
// restore-by-time.
func TestCorruptMetadata_IsSkippedInRoutineListing(t *testing.T) {
	t.Skip("BKRS-409: readBackupDetails fails the whole listing on the first unreadable metadata file")

	catalog, st, root := newLocalTestCatalog(t)
	routine := &model.BackupRoutine{Name: "r", Storage: st, BackupPolicy: &model.BackupPolicy{}}
	ps := NewPathService(nil)

	ts1 := time.UnixMilli(1_700_000_000_000).UTC()
	ts2 := ts1.Add(24 * time.Hour)
	writeTestMetadata(t, root, "r", model.BackupTypeFull, ts1, "ns1", time.Time{}, 1)

	badDir := filepath.Join(root, ps.GetBackupPath("r", model.BackupTypeFull, "ns1", ts2))
	require.NoError(t, os.MkdirAll(badDir, 0o750))

	for name, corrupt := range map[string][]byte{"truncated": []byte("created: 2023-11-1"), "empty": nil} {
		require.NoError(t, os.WriteFile(filepath.Join(badDir, metadataFile), corrupt, 0o600), name)

		backups, err := catalog.GetBackups(t.Context(), NewFullBackupFilter(routine))
		require.NoError(t, err, name)
		require.Len(t, backups, 1, name)
		assert.True(t, ts1.Equal(backups[0].Created), name)

		last, err := NewHistoryManager(catalog).FindLastRun(t.Context(), routine)
		require.NoError(t, err, name)
		assert.True(t, ts1.Equal(*last.FullBackupTime()), name)

		runner := &timeRestoreRunner{backupReader: catalog}
		chains, err := runner.findBackupsToRestore(t.Context(), &model.RestoreTimestampRequest{
			RoutineName: "r", Storage: st, Time: ts2.Add(time.Hour),
		})
		require.NoError(t, err, name)
		assert.Len(t, chains["ns1"], 1, name)
	}
}

// --- 4. Restore-by-path must refuse a folder without a completion marker ----------------

// A backup folder without metadata.yaml is a backup that never completed (the process died
// before the marker was written); restoring it "as is" silently restores a subset of the data.
func TestRestoreByPath_RejectsFolderWithoutMetadata(t *testing.T) {
	t.Skip("BKRS-410: ValidatePath accepts a folder without metadata")

	ctrl := gomock.NewController(t)
	sc := NewMockStartController(ctrl)
	rp := NewmockRoutineProvider(ctrl)
	v := NewRestoreValidator(sc, rp)

	req := &model.RestoreRequest{Policy: model.RestorePolicy{}}
	err := v.ValidatePath(t.Context(), req, nil, nil /* no metadata found under the path */)
	require.Error(t, err)
}

// --- 5. Retention with incremental=0 must delete only completed incrementals -----------

// retention.incremental = 0 means "keep no completed incrementals". Deleting <routine>/incremental
// as a whole also removes the folder of an incremental that is still running (concurrent
// incrementals are allowed), which then finishes and writes metadata over missing data.
func TestRetention_ZeroIncrementalDeletesOnlyCompletedBackups(t *testing.T) {
	t.Skip("BKRS-411: the whole incremental root is deleted")

	ctrl := gomock.NewController(t)
	catalog := NewMockBackupCatalog(ctrl)
	routine := &model.BackupRoutine{
		Name: "r",
		BackupPolicy: &model.BackupPolicy{
			RetentionPolicy: &model.RetentionPolicy{
				FullBackups: optional.Of(2),
				IncrBackups: optional.Of(0),
			},
		},
	}
	full := []model.BackupDetails{
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(1000), Namespace: "ns"}, Key: "r/backup/1000/data/ns"},
	}
	catalog.EXPECT().GetBackups(gomock.Any(), gomock.Any()).Return(full, nil).AnyTimes()

	var deleted []string
	catalog.EXPECT().Delete(gomock.Any(), routine, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ *model.BackupRoutine, p string) error {
			deleted = append(deleted, p)
			return nil
		}).AnyTimes()

	rm := NewBackupRetentionManager(catalog, &testLockMap)
	require.NoError(t, rm.ApplyRetention(t.Context(), routine))
	assert.NotContains(t, deleted, "r/incremental", "only completed (metadata-bearing) incrementals may be deleted")
}

// --- 6. Retention: multi-namespace duplicates are harmless on local storage -------

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
