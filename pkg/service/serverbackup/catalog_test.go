package serverbackup

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/aerospike/backup-go/pkg/server/lister"
	listermodels "github.com/aerospike/backup-go/pkg/server/lister/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// fakeSource serves fixed metadata and counts, and records which backups' counts were read.
type fakeSource struct {
	metadata []listermodels.Metadata
	stats    map[string]listermodels.Statistics
	statsErr error

	mu        sync.Mutex
	statsRead []string
}

func (f *fakeSource) FetchAllMetadata(context.Context) ([]listermodels.Metadata, error) {
	return f.metadata, nil
}

func (f *fakeSource) GetMetadata(_ context.Context, backupID string) (listermodels.Metadata, error) {
	i := slices.IndexFunc(f.metadata, func(m listermodels.Metadata) bool { return m.BackupID == backupID })
	if i < 0 {
		return listermodels.Metadata{}, lister.ErrMetadataNotFound
	}

	return f.metadata[i], nil
}

func (f *fakeSource) GetStatistics(_ context.Context, backupID string) (listermodels.Statistics, error) {
	f.mu.Lock()
	f.statsRead = append(f.statsRead, backupID)
	f.mu.Unlock()

	return f.stats[backupID], f.statsErr
}

func (f *fakeSource) open(context.Context, model.Storage, string) (backupSource, error) {
	return f, nil
}

func (f *fakeSource) catalog() *catalog {
	return &catalog{sources: f}
}

var catalogStart = time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)

func catalogAt(minutes int) time.Time { return catalogStart.Add(time.Duration(minutes) * time.Minute) }

// fullMetadata returns the metadata of a complete full backup sealed at created.
func fullMetadata(id, namespace string, created time.Time) listermodels.Metadata {
	return listermodels.Metadata{
		BackupID: id, Namespace: namespace, Status: listermodels.BackupStatusComplete,
		ModifiedBeforeUnixS: created.Unix(),
	}
}

// incrMetadata returns the metadata of a complete incremental backup from from to created.
func incrMetadata(id, namespace string, from, created time.Time) listermodels.Metadata {
	m := fullMetadata(id, namespace, created)
	m.ModifiedAfterUnixS = from.Unix()

	return m
}

// nonEmpty is the counts of a backup that holds records.
var nonEmpty = listermodels.Statistics{
	RecordCount: 3, IncrRecordCount: 1, ChangeRecordCount: 2, ByteCount: 300, FileCount: 4, IndexCount: 2, UDFCount: 1,
}

func keys(backups []model.BackupDetails) []string {
	keys := make([]string, 0, len(backups))
	for _, b := range backups {
		keys = append(keys, b.Key)
	}

	return keys
}

func TestCatalog_GetBackups_Counts(t *testing.T) {
	t.Parallel()

	source := &fakeSource{
		metadata: []listermodels.Metadata{fullMetadata("full", "bar", catalogAt(0))},
		stats:    map[string]listermodels.Statistics{"full": nonEmpty},
	}
	st := &model.S3Storage{Bucket: "bucket"}
	routine := &model.BackupRoutine{Name: "routine", Storage: st}

	backups, err := source.catalog().GetBackups(t.Context(), service.NewFullBackupFilter(routine))

	require.NoError(t, err)
	require.Len(t, backups, 1)
	// The record count is the base round's alone: the other two re-read records it holds.
	assert.Equal(t, model.BackupMetadata{
		Created:             catalogAt(0),
		Namespace:           "bar",
		RecordCount:         3,
		ByteCount:           300,
		FileCount:           4,
		SecondaryIndexCount: 2,
		UDFCount:            1,
		Compression:         model.CompressionModeNone,
		Encryption:          model.EncryptionModeNone,
	}, backups[0].BackupMetadata)
	assert.Equal(t, "routine/full", backups[0].Key)
}

func TestCatalog_GetBackups(t *testing.T) {
	t.Parallel()

	metadata := []listermodels.Metadata{
		fullMetadata("full-empty", "bar", catalogAt(0)),
		incrMetadata("incr-1", "bar", catalogAt(0), catalogAt(10)),
		incrMetadata("incr-2-empty", "bar", catalogAt(10), catalogAt(20)),
		{BackupID: "aborted", Namespace: "bar", Status: listermodels.BackupStatusAborted},
		fullMetadata("full", "bar", catalogAt(30)),
		incrMetadata("incr-3", "bar", catalogAt(30), catalogAt(40)),
		incrMetadata("incr-4-empty", "bar", catalogAt(40), catalogAt(50)),
	}
	stats := map[string]listermodels.Statistics{"full": nonEmpty, "incr-1": nonEmpty, "incr-3": nonEmpty}
	routine := &model.BackupRoutine{Name: "routine", Storage: &model.S3Storage{}}

	tests := []struct {
		name          string
		filter        *service.RoutineFilter
		want          []string
		wantStatsRead []string
	}{
		{
			name:          "full backups, an empty one too",
			filter:        service.NewFullBackupFilter(routine),
			want:          []string{"routine/full-empty", "routine/full"},
			wantStatsRead: []string{"full-empty", "full"},
		},
		{
			name:   "incremental backups, the empty ones too",
			filter: service.NewIncrementalBackupFilter(routine),
			want: []string{
				"routine/incr-1", "routine/incr-2-empty", "routine/incr-3", "routine/incr-4-empty",
			},
			wantStatsRead: []string{"incr-1", "incr-2-empty", "incr-3", "incr-4-empty"},
		},
		{
			name:          "counts read only within the time bounds",
			filter:        service.NewIncrementalBackupFilter(routine).WithFromTime(catalogAt(15)),
			want:          []string{"routine/incr-2-empty", "routine/incr-3", "routine/incr-4-empty"},
			wantStatsRead: []string{"incr-2-empty", "incr-3", "incr-4-empty"},
		},
		{
			name:          "last full backup",
			filter:        service.NewFullBackupFilter(routine).Last(),
			want:          []string{"routine/full"},
			wantStatsRead: []string{"full"},
		},
		{
			name:          "last incremental backup, an empty one too",
			filter:        service.NewIncrementalBackupFilter(routine).Last(),
			want:          []string{"routine/incr-4-empty"},
			wantStatsRead: []string{"incr-4-empty"},
		},
		{
			name:   "no last incremental backup out of the time bounds",
			filter: service.NewIncrementalBackupFilter(routine).WithFromTime(catalogAt(55)).Last(),
			want:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			source := &fakeSource{metadata: metadata, stats: stats}

			backups, err := source.catalog().GetBackups(t.Context(), tt.filter)

			require.NoError(t, err)
			assert.Equal(t, tt.want, keys(backups))
			assert.Equal(t, tt.wantStatsRead, source.statsRead)
		})
	}
}

// TestCatalog_AllBackups checks that every committed backup is listed and that no counts are read
// for it.
func TestCatalog_AllBackups(t *testing.T) {
	t.Parallel()

	source := &fakeSource{metadata: []listermodels.Metadata{
		fullMetadata("full", "bar", catalogAt(0)),
		incrMetadata("incr-empty", "bar", catalogAt(0), catalogAt(10)),
		{BackupID: "aborted", Namespace: "bar", Status: listermodels.BackupStatusAborted},
		fullMetadata("full-baz", "baz", catalogAt(20)),
	}}
	routine := &model.BackupRoutine{Name: "routine", Storage: &model.S3Storage{}}

	backups, err := source.catalog().AllBackups(t.Context(), routine.Storage, routine.Name)

	require.NoError(t, err)
	assert.Equal(t, []string{"routine/full", "routine/incr-empty", "routine/full-baz"}, keys(backups))
	assert.Empty(t, source.statsRead)
}

func TestCatalog_Delete(t *testing.T) {
	t.Parallel()

	backupStorage := &model.S3Storage{}
	key := "routine/" + fullID

	tests := []struct {
		name       string
		key        string
		wantDelete bool
		deleteErr  error
		wantErr    string
	}{
		{name: "the folder of the backup", key: key, wantDelete: true},
		{
			name: "the storage refuses", key: key, wantDelete: true, deleteErr: errors.New("denied"),
			wantErr: "denied",
		},
		// Without a job id the key is the folder of all the routine's backups.
		{name: "no job id", key: "routine/", wantErr: `"" is not a job id`},
		{name: "a path for a job id", key: "routine/../other", wantErr: `"../other" is not a job id`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			operations := storage.NewMockOperations(gomock.NewController(t))
			if tt.wantDelete {
				operations.EXPECT().DeleteFolder(t.Context(), backupStorage, tt.key).Return(tt.deleteErr)
			}

			c := &catalog{operations: operations}

			err := c.Delete(t.Context(), backupStorage, tt.key)

			assert.True(t, c.locks.Get("routine").TryLock(), "the routine stayed locked after the delete")
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestCatalog_Delete_LocksReads checks that no read of the routine's backups runs while one of
// them is deleted.
func TestCatalog_Delete_LocksReads(t *testing.T) {
	t.Parallel()

	operations := storage.NewMockOperations(gomock.NewController(t))
	c := &catalog{operations: operations}

	operations.EXPECT().DeleteFolder(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(context.Context, model.Storage, string) error {
			assert.False(t, c.locks.Get("routine").TryRLock(), "a read could start during the delete")

			return nil
		})

	require.NoError(t, c.Delete(t.Context(), &model.S3Storage{}, "routine/"+fullID))
}

// probedSource calls onRead whenever the catalog reads metadata from it.
type probedSource struct {
	*fakeSource
	onRead func()
}

func (p *probedSource) FetchAllMetadata(ctx context.Context) ([]listermodels.Metadata, error) {
	p.onRead()

	return p.fakeSource.FetchAllMetadata(ctx)
}

func (p *probedSource) GetMetadata(ctx context.Context, backupID string) (listermodels.Metadata, error) {
	p.onRead()

	return p.fakeSource.GetMetadata(ctx, backupID)
}

func (p *probedSource) open(context.Context, model.Storage, string) (backupSource, error) {
	return p, nil
}

// TestCatalog_Reads_LockTheRoutine checks that no backup of the routine can be deleted while its
// backups are read, and that a read leaves the routine unlocked.
func TestCatalog_Reads_LockTheRoutine(t *testing.T) {
	t.Parallel()

	routine := &model.BackupRoutine{Name: "routine", Storage: &model.S3Storage{}}

	tests := []struct {
		name string
		read func(ctx context.Context, c *catalog) error
	}{
		{name: "GetBackups", read: func(ctx context.Context, c *catalog) error {
			_, err := c.GetBackups(ctx, service.NewFullBackupFilter(routine))
			return err
		}},
		{name: "GetBackup", read: func(ctx context.Context, c *catalog) error {
			_, err := c.GetBackup(ctx, routine.Storage, "routine/"+fullID)
			return err
		}},
		{name: "AllBackups", read: func(ctx context.Context, c *catalog) error {
			_, err := c.AllBackups(ctx, routine.Storage, routine.Name)
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			source := &probedSource{fakeSource: &fakeSource{
				metadata: []listermodels.Metadata{fullMetadata(fullID, "bar", catalogAt(0))},
			}}
			c := &catalog{sources: source}
			lock := c.locks.Get("routine")
			reads := 0
			source.onRead = func() {
				reads++
				assert.False(t, lock.TryLock(), "a backup could be deleted during the read")
			}

			require.NoError(t, tt.read(t.Context(), c))

			assert.Positive(t, reads)
			assert.True(t, lock.TryLock(), "the routine stayed locked after the read")
		})
	}
}

// A backup whose counts cannot be read fails the listing rather than pass for empty.
func TestCatalog_GetBackups_StatisticsFail(t *testing.T) {
	t.Parallel()

	source := &fakeSource{
		metadata: []listermodels.Metadata{incrMetadata("incr", "bar", catalogAt(0), catalogAt(10))},
		statsErr: errors.New("storage down"),
	}
	routine := &model.BackupRoutine{Name: "routine", Storage: &model.S3Storage{}}

	for _, filter := range []*service.RoutineFilter{
		service.NewIncrementalBackupFilter(routine),
		service.NewIncrementalBackupFilter(routine).Last(),
	} {
		backups, err := source.catalog().GetBackups(t.Context(), filter)

		require.EqualError(t, err, "failed to read the counts of server backup incr: storage down")
		assert.Nil(t, backups)
	}
}

func TestCatalog_GetBackup(t *testing.T) {
	t.Parallel()

	source := &fakeSource{
		metadata: []listermodels.Metadata{incrMetadata("incr", "bar", catalogAt(0), catalogAt(10))},
		stats:    map[string]listermodels.Statistics{"incr": nonEmpty},
	}

	backup, err := source.catalog().GetBackup(t.Context(), &model.S3Storage{}, "routine/incr")

	require.NoError(t, err)
	assert.Equal(t, "routine/incr", backup.Key)
	assert.Equal(t, catalogAt(0), backup.From)
	assert.Equal(t, catalogAt(10), backup.Created)
	assert.Empty(t, source.statsRead, "restore reads no counts")
}

func TestFinishedTime(t *testing.T) {
	t.Parallel()

	// 2026-11-18T12:00:00.250Z in Citrusleaf epoch ms (since 2010-01-01).
	const consistentMs = 532699200250
	want := time.Date(2026, 11, 18, 12, 0, 0, 250_000_000, time.UTC)

	tests := []struct {
		name     string
		metadata listermodels.Metadata
		want     time.Time
	}{
		{name: "strict backup at its checkpoint", metadata: listermodels.Metadata{CheckpointMs: consistentMs}, want: want},
		{name: "fuzzy backup at the end of its base scan",
			metadata: listermodels.Metadata{BaseScanEndMs: consistentMs}, want: want},
		{name: "neither recorded"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, finishedTime(tt.metadata))
		})
	}
}
