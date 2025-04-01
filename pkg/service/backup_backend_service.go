package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	"gopkg.in/yaml.v3"
)

// BackupFilter is an interface that all filter types must implement.
type BackupFilter interface {
	isBackupFilter()
}

// BaseFilter contains common filter attributes.
type BaseFilter struct {
	FromTime *time.Time
	ToTime   *time.Time
	onlyLast bool // last backup only
}

// RoutineFilter for filtering by routine and job type.
type RoutineFilter struct {
	BaseFilter
	routine string
	JobType jobType
}

// NewFullBackupFilter creates a filter for full backups.
func NewFullBackupFilter(routine string) *RoutineFilter {
	return &RoutineFilter{
		routine: routine,
		JobType: jobTypeFull,
	}
}

// NewIncrementalBackupFilter creates a filter for incremental backups.
func NewIncrementalBackupFilter(routine string) *RoutineFilter {
	return &RoutineFilter{
		routine: routine,
		JobType: jobTypeIncremental,
	}
}

func (f *RoutineFilter) isBackupFilter() {}

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

func (f RoutineFilter) getUpperBoundary() string {
	if f.ToTime != nil {
		return getTimestampPath(f.routine, *f.ToTime, f.JobType)
	}

	return "\uffff"
}

func (f *RoutineFilter) String() string {
	return fmt.Sprintf("routine: %v type: %v last: %v timebounds: %s",
		f.routine, f.JobType, f.onlyLast, f.TimeBounds().String())
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

// Implement BackupFilter interface for both concrete types.
func (p *PathFilter) isBackupFilter() {}

// TimeBounds returns time bounds for a base filter.
func (b *BaseFilter) TimeBounds() *model.TimeBounds {
	return &model.TimeBounds{
		FromTime: b.FromTime,
		ToTime:   b.ToTime,
	}
}

func (p *PathFilter) WithToTime(toTime time.Time) *PathFilter {
	p.ToTime = &toTime
	return p
}

func (f *RoutineFilter) WithTimeBounds(bounds model.TimeBounds) *RoutineFilter {
	f.FromTime = bounds.FromTime
	f.ToTime = bounds.ToTime
	return f
}

func (p *PathFilter) WithTimeBounds(bounds model.TimeBounds) *PathFilter {
	p.FromTime = bounds.FromTime
	p.ToTime = bounds.ToTime
	return p
}

func (p *PathFilter) String() string {
	return fmt.Sprintf("path: %v storage: %s last: %v timebounds: %s",
		p.path, p.storage.String(), p.onlyLast, p.TimeBounds().String())
}

// GetPath methods.
func (f *RoutineFilter) getPath() string {
	return getBackupRootPath(f.routine, f.JobType)
}

func (p *PathFilter) getPath() string {
	return p.path
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
	WriteBackupMetadata(ctx context.Context, routineName, path string, metadata model.BackupMetadata) error

	// Delete removes a specific backup folder.
	Delete(ctx context.Context, routineName, path string) error
}

// BackupBackendServiceImpl default implementation of BackupReaderWriter.
type BackupBackendServiceImpl struct {
	config *model.Config
	locks  *util.SafeMap[string, *sync.RWMutex] // lock per routine
}

var _ BackupReaderWriter = (*BackupBackendServiceImpl)(nil)

func NewBackupBackendService(config *model.Config) *BackupBackendServiceImpl {
	return &BackupBackendServiceImpl{
		config: config,
		locks:  util.NewSafeMap[string, *sync.RWMutex](),
	}
}

func (b *BackupBackendServiceImpl) GetBackups(ctx context.Context, filter BackupFilter) ([]model.BackupDetails, error) {
	switch f := filter.(type) {
	case *RoutineFilter:
		return b.getRoutineBackups(ctx, *f)
	case *PathFilter:
		return b.getPathBackups(ctx, *f)
	default:
		return nil, fmt.Errorf("unsupported filter type: %T", f)
	}
}

func (b *BackupBackendServiceImpl) getRoutineBackups(
	ctx context.Context,
	filter RoutineFilter,
) ([]model.BackupDetails, error) {
	routine, found := b.config.Routine(filter.routine)
	if !found {
		return nil, fmt.Errorf("routine not found: %q", filter.routine)
	}

	backupStorage := routine.Storage
	lock := b.locks.LoadOrStore(filter.routine, &sync.RWMutex{})
	lock.RLock()
	defer lock.RUnlock()

	files, err := storage.ReadFileNames(ctx, backupStorage, filter.getPath(), metadataFile, filter.FromTime)
	if err != nil {
		return nil, fmt.Errorf("read metadata files in %s: %w", filter.FromTime, err)
	}

	// Storage returned all files >= fromTime. We need to find the one with highest timestamp that's still < ToTime.
	// We use the timestamps that are part of file path.
	maxString := filter.getUpperBoundary()

	// Filter files based on timestamp criteria
	storagePrefix := filepath.Clean(backupStorage.GetPath())
	eligibleFiles := filterEligibleFiles(files, filepath.Join(storagePrefix, maxString), filter)

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
		if filter.TimeBounds().Contains(metadata.Created) {
			key := backupKey(fileName, storagePrefix)
			details := model.NewBackupDetails(*metadata, key, backupStorage, filter.routine)
			backups = append(backups, details)
		}
	}

	return backups, nil
}

func backupKey(fileName, storagePrefix string) string {
	/* backup key is a substring between root path and metadata file name.
	fileName example: "storage/test-routine/backup/1609632000000/data/test-ns/metadata.yaml"
	                   |------|----------------------------------------------|------------|
	                   Storage|                   Backup Key                 |    Filename
	                   prefix |                                              |    (metadata.yaml)
	*/
	return strings.Trim(strings.TrimPrefix(filepath.Dir(fileName), storagePrefix), "/")
}

// filterEligibleFiles returns files that meet the timestamp criteria.
func filterEligibleFiles(files []string, maxString string, filter RoutineFilter) []string {
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

	highestTimestamp := findHighestTimestamp(lessThenMaxString)

	// Filter further to keep only files with the highest timestamp
	var latestFiles []string
	for _, fileName := range lessThenMaxString {
		if extractTimestampFromPath(fileName) == highestTimestamp {
			latestFiles = append(latestFiles, fileName)
		}
	}

	return latestFiles
}

func findHighestTimestamp(files []string) string {
	var highestTimestamp string
	for _, fileName := range files {
		timestamp := extractTimestampFromPath(fileName)
		if timestamp > highestTimestamp {
			highestTimestamp = timestamp
		}
	}
	return highestTimestamp
}

func (b *BackupBackendServiceImpl) WriteBackupMetadata(
	ctx context.Context,
	routineName string,
	path string,
	metadata model.BackupMetadata,
) error {
	routine, ok := b.config.Routine(routineName)
	if !ok {
		return fmt.Errorf("routine not found: %q", routineName)
	}

	dataYaml, err := yaml.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	metadataFilePath := filepath.Join(path, metadataFile)

	lock := b.locks.LoadOrStore(routineName, &sync.RWMutex{})
	lock.Lock()
	defer lock.Unlock()

	return storage.WriteMetadataFile(ctx, routine.Storage, metadataFilePath, dataYaml)
}

func (b *BackupBackendServiceImpl) Delete(ctx context.Context, routineName string, path string) error {
	routine, ok := b.config.Routine(routineName)
	if !ok {
		return fmt.Errorf("routine not found: %q", routineName)
	}

	lock := b.locks.LoadOrStore(routineName, &sync.RWMutex{})
	lock.Lock()
	defer lock.Unlock()

	return storage.DeleteFolder(ctx, routine.Storage, path)
}

func (b *BackupBackendServiceImpl) getPathBackups(
	ctx context.Context,
	filter PathFilter,
) ([]model.BackupDetails, error) {
	files, err := storage.ReadFileNames(ctx, filter.storage, filter.getPath(), metadataFile, nil)
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

		if filter.TimeBounds().Contains(metadata.Created) {
			key := backupKey(fileName, storagePrefix)
			details := model.NewBackupDetails(*metadata, key, filter.storage, "")
			backups = append(backups, details)
		}
	}

	return backups, nil
}
