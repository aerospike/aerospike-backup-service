package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"gopkg.in/yaml.v3"
)

// BackupFilter is an interface that all filter types must implement.
type BackupFilter interface {
	// timeBounds returns time bounds specified for current filter.
	timeBounds() model.TimeBounds
}

// BaseFilter contains common filter attributes.
type BaseFilter struct {
	FromTime *time.Time
	ToTime   *time.Time
}

// RoutineFilter for filtering by routine and job type.
type RoutineFilter struct {
	BaseFilter
	routine *model.BackupRoutine
	JobType jobType

	onlyLast bool // return last backup only
}

// NewFullBackupFilter creates a filter for full backups.
func NewFullBackupFilter(routine *model.BackupRoutine) *RoutineFilter {
	return &RoutineFilter{
		routine: routine,
		JobType: jobTypeFull,
	}
}

// NewIncrementalBackupFilter creates a filter for incremental backups.
func NewIncrementalBackupFilter(routine *model.BackupRoutine) *RoutineFilter {
	return &RoutineFilter{
		routine: routine,
		JobType: jobTypeIncremental,
	}
}

func (f *RoutineFilter) Last() *RoutineFilter {
	f.onlyLast = true
	return f
}

func (f *RoutineFilter) WithFromTime(fromTime time.Time) *RoutineFilter {
	f.FromTime = &fromTime
	return f
}

func (p *PathFilter) WithFromTime(fromTime time.Time) *PathFilter {
	p.FromTime = &fromTime
	return p
}

func (f *RoutineFilter) WithToTime(toTime time.Time) *RoutineFilter {
	f.ToTime = &toTime
	return f
}

func (f *RoutineFilter) getUpperBoundary(pathService PathService) string {
	if f.ToTime != nil {
		return pathService.GetTimestampPath(f.routine.Name, *f.ToTime, f.JobType)
	}

	return "\uffff"
}

func (f *RoutineFilter) WithTimeBounds(bounds model.TimeBounds) *RoutineFilter {
	f.FromTime = bounds.FromTime
	f.ToTime = bounds.ToTime
	return f
}

func (f *RoutineFilter) String() string {
	return fmt.Sprintf("routine: %v type: %v last: %v timebounds: %s",
		f.routine, f.JobType, f.onlyLast, f.timeBounds().String())
}

func (f *RoutineFilter) getPath() string {
	return backupRootPath(f.routine.Name, f.JobType)
}

// PathFilter for filtering by explicit path.
type PathFilter struct {
	BaseFilter
	path    string
	storage model.Storage
}

// NewPathFilter creates a filter for retrieving backups directly from a given storage path.
func NewPathFilter(path string, storage model.Storage) *PathFilter {
	return &PathFilter{
		path:    path,
		storage: storage,
	}
}

// TimeBounds returns time bounds for a base filter.
func (b *BaseFilter) timeBounds() model.TimeBounds {
	return model.TimeBounds{
		FromTime: b.FromTime,
		ToTime:   b.ToTime,
	}
}

func (p *PathFilter) WithToTime(toTime time.Time) *PathFilter {
	p.ToTime = &toTime
	return p
}

func (p *PathFilter) WithTimeBounds(bounds model.TimeBounds) *PathFilter {
	p.FromTime = bounds.FromTime
	p.ToTime = bounds.ToTime
	return p
}

func (p *PathFilter) String() string {
	return fmt.Sprintf("path: %v storage: %s timebounds: %s",
		p.path, p.storage.String(), p.timeBounds().String())
}

// BackupReaderWriter defines operations for reading and writing backups metadata.
type BackupReaderWriter interface {
	BackupReader
	BackupWriter
}

// BackupReader defines operations for reading backups metadata.
type BackupReader interface {
	// GetBackups retrieves backup details based on the provided filter.
	GetBackups(ctx context.Context, filter BackupFilter) ([]model.BackupDetails, error)
}

// BackupWriter defines operations for writing backups metadata.
type BackupWriter interface {
	// WriteBackupMetadata stores metadata for a specific backup.
	WriteBackupMetadata(context.Context, *model.BackupRoutine, string, model.BackupMetadata) error

	// Delete removes a specific backup folder.
	Delete(ctx context.Context, routine *model.BackupRoutine, path string) error
}

var ErrNotFound = errors.New("not found")

// BackupBackendServiceImpl default implementation of BackupReaderWriter.
type BackupBackendServiceImpl struct {
	config      *model.Config
	locks       collections.LockMap // lock per routine
	pathService PathService
}

var _ BackupReaderWriter = (*BackupBackendServiceImpl)(nil)

func NewBackupBackendService(config *model.Config, pathService PathService) *BackupBackendServiceImpl {
	return &BackupBackendServiceImpl{
		config:      config,
		pathService: pathService,
	}
}

func (b *BackupBackendServiceImpl) GetBackups(ctx context.Context, filter BackupFilter) ([]model.BackupDetails, error) {
	switch f := filter.(type) {
	case *RoutineFilter:
		return b.getRoutineBackups(ctx, f)
	case *PathFilter:
		return b.getPathBackups(ctx, f)
	default:
		return nil, fmt.Errorf("unsupported filter type: %T", f)
	}
}

func (b *BackupBackendServiceImpl) getRoutineBackups(
	ctx context.Context,
	filter *RoutineFilter,
) ([]model.BackupDetails, error) {
	backupStorage := filter.routine.Storage
	lock := b.locks.Get(filter.routine.Name)
	lock.RLock()
	defer lock.RUnlock()

	files, err := storage.ReadFileNames(ctx, backupStorage, filter.getPath(), metadataFile, filter.FromTime)
	if err != nil {
		return nil, fmt.Errorf("read metadata files in %s: %w", filter.FromTime, err)
	}

	// Storage returned all files >= fromTime. We need to find the one with the highest timestamp that's still < ToTime.
	// We use the timestamps that are part of file path.
	maxString := filter.getUpperBoundary(b.pathService)

	// Filter files based on timestamp criteria
	storagePrefix := filepath.Clean(backupStorage.GetPath())
	eligibleFiles := b.filterEligibleFiles(files, filepath.Join(storagePrefix, maxString), filter)

	var backups []model.BackupDetails
	for _, fileName := range eligibleFiles {
		file, err := storage.ReadFile(ctx, backupStorage, strings.TrimPrefix(fileName, storagePrefix))
		if err != nil {
			return nil, fmt.Errorf("read metadata file %q: %w", fileName, err)
		}
		metadata, err := model.NewMetadataFromBytes(file)
		if err != nil {
			return nil, fmt.Errorf("error decoding backup metadata YAML: %w", err)
		}
		if filter.timeBounds().Contains(metadata.Created) {
			key := backupKey(fileName, storagePrefix)
			details := model.NewBackupDetails(*metadata, key, backupStorage)
			backups = append(backups, details)
		}
	}

	return backups, nil
}

// filterEligibleFiles returns files that meet the timestamp criteria.
func (b *BackupBackendServiceImpl) filterEligibleFiles(
	files []string,
	maxString string,
	filter *RoutineFilter,
) []string {
	var lessThenMaxString []string
	for _, fileName := range files {
		if fileName < maxString || strings.HasPrefix(fileName, maxString) {
			// hasPrefix conditions is required to filter in exact timestamp
			lessThenMaxString = append(lessThenMaxString, fileName)
		}
	}

	if !filter.onlyLast {
		return lessThenMaxString
	}

	highestTimestamp := b.findHighestTimestamp(lessThenMaxString)

	// Filter further to keep only files with the highest timestamp
	var latestFiles []string
	for _, fileName := range lessThenMaxString {
		if b.pathService.ExtractTimestampFromPath(fileName) == highestTimestamp {
			latestFiles = append(latestFiles, fileName)
		}
	}

	return latestFiles
}

func (b *BackupBackendServiceImpl) findHighestTimestamp(files []string) string {
	var highestTimestamp string
	for _, fileName := range files {
		timestamp := b.pathService.ExtractTimestampFromPath(fileName)
		if timestamp > highestTimestamp {
			highestTimestamp = timestamp
		}
	}
	return highestTimestamp
}

func (b *BackupBackendServiceImpl) WriteBackupMetadata(
	ctx context.Context,
	routine *model.BackupRoutine,
	path string,
	metadata model.BackupMetadata,
) error {
	dataYaml, err := yaml.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	metadataFilePath := filepath.Join(path, metadataFile)

	lock := b.locks.Get(routine.Name)
	lock.Lock()
	defer lock.Unlock()

	return storage.WriteMetadataFile(ctx, routine.Storage, metadataFilePath, dataYaml)
}

func (b *BackupBackendServiceImpl) Delete(ctx context.Context, routine *model.BackupRoutine, path string) error {
	lock := b.locks.Get(routine.Name)
	lock.Lock()
	defer lock.Unlock()

	err := storage.DeleteFolder(ctx, routine.Storage, path)
	if err != nil {
		return fmt.Errorf("failed to delete folder: %w", err)
	}

	slog.Info("Deleted folder", slog.String("path", path), attr.Routine(routine.Name))

	return err
}

func (b *BackupBackendServiceImpl) getPathBackups(
	ctx context.Context,
	filter *PathFilter,
) ([]model.BackupDetails, error) {
	files, err := storage.ReadFileNames(ctx, filter.storage, filter.path, metadataFile, nil)
	if err != nil {
		return nil, fmt.Errorf("read metadata files in %s: %w", filter.String(), err)
	}

	storagePrefix := filepath.Clean(filter.storage.GetPath())
	var backups []model.BackupDetails
	for _, fileName := range files {
		file, err := storage.ReadFile(ctx, filter.storage, strings.TrimPrefix(fileName, storagePrefix))
		if err != nil {
			return nil, fmt.Errorf("read metadata file %q: %w", fileName, err)
		}
		metadata, err := model.NewMetadataFromBytes(file)
		if err != nil {
			return nil, fmt.Errorf("error decoding backup metadata YAML: %w", err)
		}

		if filter.timeBounds().Contains(metadata.Created) {
			key := backupKey(fileName, storagePrefix)
			details := model.NewBackupDetails(*metadata, key, filter.storage)
			backups = append(backups, details)
		}
	}

	return backups, nil
}
