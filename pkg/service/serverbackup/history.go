package serverbackup

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/aerospike/backup-go/pkg/server"
	servermodels "github.com/aerospike/backup-go/pkg/server/models"
)

const citrusleafEpoch = 1262304000

// MetadataLister reads server backup metadata from object storage.
type MetadataLister interface {
	FetchAllMetadata(ctx context.Context) ([]servermodels.Metadata, error)
	GetMetadata(ctx context.Context, backupID string) (servermodels.Metadata, error)
}

type listerFactory interface {
	NewLister(ctx context.Context, storage model.Storage) (MetadataLister, error)
}

// ListerFactory builds metadata listers for backup storage.
type ListerFactory interface {
	NewLister(ctx context.Context, storage model.Storage) (MetadataLister, error)
}

// HistoryManager reads last-run timestamps from server backup metadata in S3.
type HistoryManager struct {
	listerFactory listerFactory
}

// NewHistoryManager builds a [HistoryManager].
func NewHistoryManager(listerFactory listerFactory) *HistoryManager {
	return &HistoryManager{listerFactory: listerFactory}
}

// FindLastRun lists server backup metadata and derives the latest run time.
func (hm *HistoryManager) FindLastRun(
	ctx context.Context,
	routine *model.BackupRoutine,
) (*model.BackupTime, error) {
	lister, err := hm.listerFactory.NewLister(ctx, routine.Storage)
	if err != nil {
		return nil, err
	}

	all, err := lister.FetchAllMetadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("list server backup metadata failed: %w", err)
	}

	lastRun := lastBackupTime(routine, all)

	slog.Debug("Last server backup time completed for routine",
		attr.Routine(routine.Name),
		slog.String("lastRun", lastRun.String()))

	return lastRun, nil
}

func lastBackupTime(routine *model.BackupRoutine, all []servermodels.Metadata) *model.BackupTime {
	if len(all) == 0 {
		return model.NewNoBackupTime()
	}

	latestTime := metadataFinishedTime(all[len(all)-1])
	if routine.IncrIntervalCron == "" || len(all) == 1 {
		return model.NewFullBackupTime(latestTime)
	}

	prevTime := metadataFinishedTime(all[len(all)-2])
	if latestTime.After(prevTime) {
		return model.NewBackupTime(prevTime, latestTime)
	}

	return model.NewFullBackupTime(latestTime)
}

func metadataFinishedTime(md servermodels.Metadata) time.Time {
	var latest time.Time
	for _, node := range md.Nodes {
		if node.Finished.After(latest) {
			latest = node.Finished
		}
	}
	if !latest.IsZero() {
		return latest
	}

	return backupIDToTime(md.BackupID)
}

func backupIDToTime(backupID string) time.Time {
	ts, err := strconv.ParseInt(backupID, 10, 64)
	if err != nil {
		return time.Time{}
	}

	return time.Unix(ts+citrusleafEpoch, 0)
}

type s3ListerFactory struct {
	s3Client s3ClientProvider
}

var _ listerFactory = (*s3ListerFactory)(nil)
var _ ListerFactory = (*s3ListerFactory)(nil)

type s3ClientProvider interface {
	GetS3Client(ctx context.Context, storage *model.S3Storage) (server.S3API, error)
}

// NewS3ListerFactory builds listers backed by backup-go's server.Lister.
func NewS3ListerFactory(s3Client s3ClientProvider) *s3ListerFactory {
	return &s3ListerFactory{s3Client: s3Client}
}

func (f *s3ListerFactory) NewLister(
	ctx context.Context,
	storage model.Storage,
) (MetadataLister, error) {
	s3Storage, ok := storage.(*model.S3Storage)
	if !ok {
		return nil, fmt.Errorf("server backup history requires S3 storage, got %T", storage)
	}
	client, err := f.s3Client.GetS3Client(ctx, s3Storage)
	if err != nil {
		return nil, fmt.Errorf("failed to get S3 client: %w", err)
	}

	return server.NewLister(client, s3Storage.Bucket, s3Storage.Path, server.WithLogger(slog.Default())), nil
}

type s3StorageClientProvider struct {
	accessor *storage.S3StorageAccessor
}

var _ s3ClientProvider = (*s3StorageClientProvider)(nil)

// NewS3StorageClientProvider adapts [storage.S3StorageAccessor] for server backup history.
func NewS3StorageClientProvider(accessor *storage.S3StorageAccessor) *s3StorageClientProvider {
	return &s3StorageClientProvider{accessor: accessor}
}

func (p *s3StorageClientProvider) GetS3Client(
	ctx context.Context,
	s *model.S3Storage,
) (server.S3API, error) {
	return p.accessor.GetClient(ctx, s)
}
