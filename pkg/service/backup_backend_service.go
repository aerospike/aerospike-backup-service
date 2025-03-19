package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
)

// BackupFilter defines criteria for filtering backups.
type BackupFilter struct {
	// required
	routine string
	JobType jobType

	// optional
	FromTime *time.Time
	ToTime   *time.Time
}

// NewFullBackupFilter creates a filter for full backups.
func NewFullBackupFilter(routine string) BackupFilter {
	return BackupFilter{
		routine: routine,
		JobType: jobTypeFull,
	}
}

// NewIncrementalBackupFilter creates a filter for incremental backups.
func NewIncrementalBackupFilter(routine string) BackupFilter {
	return BackupFilter{
		routine: routine,
		JobType: jobTypeIncremental,
	}
}

func (f BackupFilter) TimeBounds() *model.TimeBounds {
	return &model.TimeBounds{
		FromTime: f.FromTime,
		ToTime:   f.ToTime,
	}
}

// WithFromTime adds a start time to the filter.
func (f BackupFilter) WithFromTime(fromTime time.Time) BackupFilter {
	f.FromTime = &fromTime
	return f
}

func (f BackupFilter) WithTimebounds(bounds model.TimeBounds) BackupFilter {
	f.FromTime = bounds.FromTime
	f.ToTime = bounds.ToTime
	return f
}

// WithToTime adds an end time to the filter.
func (f BackupFilter) WithToTime(toTime time.Time) BackupFilter {
	f.ToTime = &toTime
	return f
}

type BackupBackendService interface {
	GetBackups(context.Context, BackupFilter) ([]model.BackupDetails, error)
}

type BackupBackendServiceImpl struct {
	config *model.Config
}

func NewBackupBackendService(config *model.Config) *BackupBackendServiceImpl {
	return &BackupBackendServiceImpl{config: config}
}

func (b *BackupBackendServiceImpl) GetBackups(ctx context.Context, filter BackupFilter) ([]model.BackupDetails, error) {
	path := getBackupRootPath(filter.routine, filter.JobType)

	routine, _ := b.config.Routine(filter.routine)

	// TODO: support local files
	files, err := storage.ReadFileNames(ctx, routine.Storage, path, metadataFile, filter.FromTime)
	if err != nil {
		return nil, fmt.Errorf("read metadata files in %s: %w", filter.FromTime, err)
	}

	// Storage returned all files >= fromTime. We need to find the one with highest timestamp that's still < ToTime.
	// We use the timestamps that are part of file path.
	maxString := "\uffff"
	if filter.ToTime != nil {
		maxString = getTimestampPath(filter.routine, *filter.ToTime, filter.JobType)
	}

	var backups []model.BackupDetails
	for _, fileName := range files {
		if fileName > maxString {
			continue
		}

		file, err := storage.ReadFile(ctx, routine.Storage, strings.TrimPrefix(fileName, routine.Storage.GetPath()))
		if err != nil {
			return nil, fmt.Errorf("read metadata file %q: %w", fileName, err)
		}
		metadata, err := model.NewMetadataFromBytes(file)
		if err != nil {
			return nil, fmt.Errorf("error decoding backup metadata YAML: %w", err)
		}
		if filter.TimeBounds().Contains(metadata.Created) {
			key := getKey(filter.routine, filter.JobType, metadata)
			details := model.NewBackupDetails(*metadata, key, routine.Storage, filter.routine)
			backups = append(backups, details)
		}
	}

	return backups, nil
}
