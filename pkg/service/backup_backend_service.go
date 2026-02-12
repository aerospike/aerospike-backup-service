package service

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"gopkg.in/yaml.v3"
)

type storageOperations interface {
	// ReadFileNames lists the names of files in the specified storage matching the filter.
	ReadFileNames(ctx context.Context, storage model.Storage, path string, filterStr string, fromTime *time.Time,
	) ([]string, error)
	// ReadFile reads the content of a file in the specified storage.
	ReadFile(ctx context.Context, storage model.Storage, filepath string) ([]byte, error)
	// WriteMetadataFile writes a metadata file to the specified storage.
	WriteMetadataFile(ctx context.Context, storage model.Storage, fileName string, content []byte) error
	// DeleteFolder deletes a folder and its contents in the specified storage.
	DeleteFolder(ctx context.Context, storage model.Storage, path string) error
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

// BackupBackendServiceImpl default implementation of BackupReaderWriter.
type BackupBackendServiceImpl struct {
	config      *model.Config
	locks       collections.LockMap // lock per routine
	pathService PathService
	operations  storageOperations
}

var _ BackupReaderWriter = (*BackupBackendServiceImpl)(nil)

func NewBackupBackendService(
	config *model.Config,
	pathService PathService,
	operations storageOperations,
) *BackupBackendServiceImpl {
	return &BackupBackendServiceImpl{
		config:      config,
		pathService: pathService,
		operations:  operations,
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

	files, err := b.operations.ReadFileNames(ctx, backupStorage, filter.getPath(), metadataFile, filter.FromTime)
	if err != nil {
		return nil, fmt.Errorf("read metadata files in %s: %w", filter.getPath(), err)
	}

	// Storage returned all files >= fromTime. We need to find the one with the highest timestamp that's still < ToTime.
	// We use the timestamps that are part of file path.
	maxString := filter.getUpperBoundary(b.pathService)

	// Filter files based on timestamp criteria
	storagePrefix := filepath.Clean(backupStorage.GetPath())
	eligibleFiles := b.filterEligibleFiles(files, filepath.Join(storagePrefix, maxString))

	backups, err := b.readBackupDetails(ctx, backupStorage, eligibleFiles)
	if err != nil {
		return nil, err
	}

	backups = filterBackups(backups, filter.timeBounds())

	if filter.onlyLast {
		backups = getLastBackupsByCreated(backups)
	}

	return backups, nil
}

// filterEligibleFiles returns files that meet the timestamp criteria.
func (b *BackupBackendServiceImpl) filterEligibleFiles(
	files []string,
	maxString string,
) []string {
	var lessThenMaxString []string
	for _, fileName := range files {
		if fileName < maxString || strings.HasPrefix(fileName, maxString) {
			// hasPrefix conditions is required to filter in exact timestamp
			lessThenMaxString = append(lessThenMaxString, fileName)
		}
	}

	return lessThenMaxString
}

func getLastBackupsByCreated(backups []model.BackupDetails) []model.BackupDetails {
	if len(backups) == 0 {
		return nil
	}

	latestCreated := backups[0].Created
	for _, b := range backups[1:] {
		if b.Created.After(latestCreated) {
			latestCreated = b.Created
		}
	}

	lastBackups := make([]model.BackupDetails, 0, len(backups))
	for _, b := range backups {
		if b.Created.Equal(latestCreated) {
			lastBackups = append(lastBackups, b)
		}
	}

	return lastBackups
}

func (b *BackupBackendServiceImpl) getPathBackups(
	ctx context.Context,
	filter *PathFilter,
) ([]model.BackupDetails, error) {
	files, err := b.operations.ReadFileNames(ctx, filter.storage, filter.path, metadataFile, nil)
	if err != nil {
		return nil, fmt.Errorf("read metadata files in %s: %w", filter.String(), err)
	}
	backups, err := b.readBackupDetails(ctx, filter.storage, files)
	if err != nil {
		return nil, err
	}
	return filterBackups(backups, filter.timeBounds()), nil
}

// readBackupDetails reads and parses metadata for the given files.
func (b *BackupBackendServiceImpl) readBackupDetails(
	ctx context.Context,
	storage model.Storage,
	files []string,
) ([]model.BackupDetails, error) {
	storagePrefix := filepath.Clean(storage.GetPath())

	var backups []model.BackupDetails
	for _, fileName := range files {
		file, err := b.operations.ReadFile(ctx, storage, strings.TrimPrefix(fileName, storagePrefix))
		if err != nil {
			return nil, fmt.Errorf("read metadata file %q: %w", fileName, err)
		}
		metadata, err := model.NewMetadataFromBytes(file)
		if err != nil {
			return nil, fmt.Errorf("error decoding backup metadata YAML: %w", err)
		}

		key := backupKey(fileName, storagePrefix)
		details := model.NewBackupDetails(*metadata, key, storage)
		backups = append(backups, details)
	}
	return backups, nil
}

// filterBackups filters backups by time bounds.
func filterBackups(backups []model.BackupDetails, bounds model.TimeBounds) []model.BackupDetails {
	var filtered []model.BackupDetails
	for _, b := range backups {
		if bounds.Contains(b.Created) && bounds.Contains(b.Finished) {
			filtered = append(filtered, b)
		}
	}
	return filtered
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

	return b.operations.WriteMetadataFile(ctx, routine.Storage, metadataFilePath, dataYaml)
}

func (b *BackupBackendServiceImpl) Delete(ctx context.Context, routine *model.BackupRoutine, path string) error {
	lock := b.locks.Get(routine.Name)
	lock.Lock()
	defer lock.Unlock()

	err := b.operations.DeleteFolder(ctx, routine.Storage, path)
	if err != nil {
		return fmt.Errorf("failed to delete folder: %w", err)
	}

	slog.Info("Deleted folder", slog.String("path", path), attr.Routine(routine.Name))

	return err
}
