package serverrestore

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/serverbackup"
	servermodels "github.com/aerospike/backup-go/pkg/server/models"
)

const citrusleafEpoch = 1262304000

// BackupReader lists server backup metadata for restore planning.
type BackupReader struct {
	listerFactory serverbackup.ListerFactory
	config        *model.Config
}

// NewBackupReader builds a [BackupReader] backed by server backup metadata in S3.
func NewBackupReader(listerFactory serverbackup.ListerFactory, config *model.Config) *BackupReader {
	return &BackupReader{
		listerFactory: listerFactory,
		config:        config,
	}
}

var _ service.BackupReader = (*BackupReader)(nil)

// GetBackups returns server backup metadata converted to [model.BackupDetails].
func (r *BackupReader) GetBackups(ctx context.Context, filter service.BackupFilter) ([]model.BackupDetails, error) {
	switch f := filter.(type) {
	case *service.RoutineFilter:
		return r.getRoutineBackups(ctx, f)
	case *service.PathFilter:
		return r.getPathBackups(ctx, f)
	default:
		return nil, fmt.Errorf("unsupported backup filter type %T", filter)
	}
}

func (r *BackupReader) getRoutineBackups(
	ctx context.Context,
	filter *service.RoutineFilter,
) ([]model.BackupDetails, error) {
	if filter.BackupType() == model.BackupTypeIncremental {
		return nil, nil
	}

	routine, ok := r.config.Routine(filter.RoutineName())
	if !ok {
		return nil, fmt.Errorf("routine %q not found", filter.RoutineName())
	}

	lister, err := r.listerFactory.NewLister(ctx, routine.Storage)
	if err != nil {
		return nil, err
	}

	all, err := lister.FetchAllMetadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("list server backup metadata failed: %w", err)
	}

	details := make([]model.BackupDetails, 0, len(all))
	for _, md := range all {
		backup := metadataToDetails(md, routine.Storage)
		if !matchesTimeBounds(backup, filter.TimeBounds()) {
			continue
		}
		details = append(details, backup)
	}

	if filter.OnlyLast() && len(details) > 0 {
		return []model.BackupDetails{details[len(details)-1]}, nil
	}

	return details, nil
}

func (r *BackupReader) getPathBackups(
	ctx context.Context,
	filter *service.PathFilter,
) ([]model.BackupDetails, error) {
	lister, err := r.listerFactory.NewLister(ctx, filter.Storage())
	if err != nil {
		return nil, err
	}

	md, err := lister.GetMetadata(ctx, filter.Path())
	if err != nil {
		return nil, fmt.Errorf("list server backup metadata failed: %w", err)
	}

	details := metadataToDetails(md, filter.Storage())

	return []model.BackupDetails{details}, nil
}

func metadataToDetails(md servermodels.Metadata, storage model.Storage) model.BackupDetails {
	var (
		created     time.Time
		finished    time.Time
		recordCount uint64
		byteCount   uint64
		fileCount   uint64
	)

	for _, node := range md.Nodes {
		if created.IsZero() || node.Created.Before(created) {
			created = node.Created
		}
		if node.Finished.After(finished) {
			finished = node.Finished
		}
		recordCount += uint64(node.RecordCount)
		byteCount += uint64(node.ByteCount)
		fileCount += uint64(node.SegmentCount)
	}

	if created.IsZero() {
		created = backupIDToTime(md.BackupID)
	}
	if finished.IsZero() {
		finished = created
	}

	return model.NewBackupDetails(
		model.BackupMetadata{
			Created:     created,
			Finished:    finished,
			Namespace:   md.Namespace,
			RecordCount: recordCount,
			ByteCount:   byteCount,
			FileCount:   fileCount,
			Compression: model.CompressNone,
			Encryption:  model.EncryptNone,
		},
		md.BackupID,
		storage,
	)
}

func backupIDToTime(backupID string) time.Time {
	ts, err := strconv.ParseInt(backupID, 10, 64)
	if err != nil {
		return time.Time{}
	}

	return time.Unix(ts+citrusleafEpoch, 0)
}

func matchesTimeBounds(backup model.BackupDetails, bounds model.TimeBounds) bool {
	if bounds.FromTime != nil && backup.Created.Before(*bounds.FromTime) {
		return false
	}
	if bounds.ToTime != nil && backup.Finished.After(*bounds.ToTime) {
		return false
	}

	return true
}

func pathBackupID(path string) string {
	if path == "" {
		return path
	}

	return path[strings.LastIndex(path, "/")+1:]
}
