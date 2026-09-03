package service

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
)

// BackupCatalog lists backups, stores backup metadata, and deletes backup folders.
type BackupCatalog interface {
	BackupReader
	BackupWriter
}

// BackupReader lists backups by reading the metadata files stored inside backup folders.
type BackupReader interface {
	// GetBackups retrieves backup details based on the provided filter.
	// Returned backups are sorted by Created time in ascending order.
	GetBackups(ctx context.Context, filter BackupFilter) ([]model.BackupDetails, error)
}

// BackupWriter stores backup metadata and deletes backup folders.
type BackupWriter interface {
	// WriteBackupMetadata stores metadata for a specific backup.
	WriteBackupMetadata(context.Context, *model.BackupRoutine, string, model.BackupMetadata) error

	// Delete removes a specific backup folder.
	Delete(ctx context.Context, routine *model.BackupRoutine, path string) error
}

// backupCatalog reads and writes backup metadata, taking a lock per routine so that
// listing, writing, and deleting a routine's backups do not overlap.
type backupCatalog struct {
	locks       collections.LockMap // lock per routine
	pathService PathService
	operations  storage.Operations
}

var _ BackupCatalog = (*backupCatalog)(nil)

func NewBackupCatalog(
	pathService PathService,
	operations storage.Operations,
) BackupCatalog {
	return &backupCatalog{
		pathService: pathService,
		operations:  operations,
	}
}

func (c *backupCatalog) GetBackups(ctx context.Context, filter BackupFilter) ([]model.BackupDetails, error) {
	switch f := filter.(type) {
	case *RoutineFilter:
		lock := c.locks.Get(f.routineName)
		lock.RLock()
		defer lock.RUnlock()
		return c.getRoutineBackups(ctx, f)
	case *PathFilter:
		return c.getPathBackups(ctx, f)
	default:
		return nil, fmt.Errorf("unsupported filter type: %T", f)
	}
}

func (c *backupCatalog) WriteBackupMetadata(
	ctx context.Context,
	routine *model.BackupRoutine,
	path string,
	metadata model.BackupMetadata,
) error {
	dataYaml, err := decoder.Marshal(metadata, decoder.YAML, false)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	metadataFilePath := filepath.Join(path, metadataFile)

	lock := c.locks.Get(routine.Name)
	lock.Lock()
	defer lock.Unlock()

	return c.operations.WriteMetadataFile(ctx, routine.Storage, metadataFilePath, dataYaml)
}

func (c *backupCatalog) Delete(ctx context.Context, routine *model.BackupRoutine, path string) error {
	lock := c.locks.Get(routine.Name)
	lock.Lock()
	defer lock.Unlock()

	err := c.operations.DeleteFolder(ctx, routine.Storage, path)
	if err != nil {
		return fmt.Errorf("failed to delete folder: %w", err)
	}

	slog.Info("Deleted folder", slog.String("path", path), attr.Routine(routine.Name))

	return err
}

func (c *backupCatalog) getRoutineBackups(ctx context.Context, filter *RoutineFilter) ([]model.BackupDetails, error) {
	backupStorage := filter.storage

	files, err := c.operations.ReadFileNames(ctx, backupStorage, filter.getPath(), metadataFile, filter.FromTime)
	if err != nil {
		return nil, fmt.Errorf("read metadata files in %s: %w", filter.getPath(), err)
	}

	storagePrefix := filepath.Clean(backupStorage.GetPath())
	files = pathsRelativeToStorage(files, storagePrefix)
	maxPath := filter.getUpperBoundary(c.pathService)
	eligibleFiles := c.filterEligibleFiles(files, maxPath)

	if filter.onlyLast {
		return c.readLatestBackupDetails(ctx, backupStorage, filter, eligibleFiles)
	}

	backups, err := c.readBackupDetails(ctx, backupStorage, eligibleFiles)
	if err != nil {
		return nil, err
	}

	backups = c.filterAndSortBackups(backups, filter.timeBounds())

	return backups, nil
}

func (c *backupCatalog) readLatestBackupDetails(
	ctx context.Context,
	storage model.Storage,
	filter *RoutineFilter,
	files []string,
) ([]model.BackupDetails, error) {
	bounds := filter.timeBounds()
	pathsByTimestamp := c.groupMetadataPathsByTimestamp(files)

	for len(pathsByTimestamp) > 0 {
		// The storage layout is routine/backup/<timestamp>/data/<namespace>/metadata.yaml,
		// so the newest timestamp directory identifies the last backup run.
		timestamp := maxTimestamp(pathsByTimestamp)
		paths := pathsByTimestamp[timestamp]
		slices.Sort(paths)

		backups, err := c.readBackupDetails(ctx, storage, paths)
		if err != nil {
			return nil, err
		}

		// Created is represented by the timestamp path, but Finished only exists
		// in metadata. If this timestamp does not match the full bounds, try the
		// next newest timestamp.
		backups = c.filterAndSortBackups(backups, bounds)
		if len(backups) > 0 {
			return backups, nil
		}

		delete(pathsByTimestamp, timestamp)
	}

	return nil, nil
}

func (c *backupCatalog) getPathBackups(ctx context.Context, filter *PathFilter) ([]model.BackupDetails, error) {
	files, err := c.operations.ReadFileNames(ctx, filter.storage, filter.path, metadataFile, nil)
	if err != nil {
		return nil, fmt.Errorf("read metadata files in %s: %w", filter.String(), err)
	}
	files = pathsRelativeToStorage(files, filepath.Clean(filter.storage.GetPath()))
	backups, err := c.readBackupDetails(ctx, filter.storage, files)
	if err != nil {
		return nil, err
	}
	return c.filterAndSortBackups(backups, filter.timeBounds()), nil
}

// pathsRelativeToStorage normalizes paths from ReadFileNames to be relative to storage root.
// ReadFile expects paths relative to storage; this strips the storage prefix when present.
func pathsRelativeToStorage(files []string, storagePrefix string) []string {
	out := make([]string, 0, len(files))
	for _, p := range files {
		rel := strings.Trim(strings.TrimPrefix(p, storagePrefix), "/")
		out = append(out, rel)
	}
	return out
}

// filterEligibleFiles returns file paths that are at or before the path upper bound.
// File paths and maxPath are relative to storage root (same format ReadFile expects).
func (c *backupCatalog) filterEligibleFiles(files []string, maxPath string) []string {
	out := make([]string, 0, len(files))
	for _, p := range files {
		if p < maxPath || strings.HasPrefix(p, maxPath) {
			out = append(out, p)
		}
	}
	return out
}

func (c *backupCatalog) groupMetadataPathsByTimestamp(files []string) map[string][]string {
	pathsByTimestamp := make(map[string][]string)
	for _, file := range files {
		timestamp := c.pathService.ExtractTimestampFromPath(file)
		if timestamp != "" {
			pathsByTimestamp[timestamp] = append(pathsByTimestamp[timestamp], file)
		}
	}

	return pathsByTimestamp
}

func maxTimestamp(pathsByTimestamp map[string][]string) string {
	var latestTimestamp string
	for timestamp := range pathsByTimestamp {
		latestTimestamp = max(latestTimestamp, timestamp)
	}
	return latestTimestamp
}

// readBackupDetails reads and parses metadata for the given metadata file paths.
// Paths must be relative to storage root (same format as ReadFileNames returns).
// Callers must pass only paths for completed backups (metadata exists and Finished is set).
func (c *backupCatalog) readBackupDetails(
	ctx context.Context,
	storage model.Storage,
	files []string,
) ([]model.BackupDetails, error) {
	backups := make([]model.BackupDetails, 0, len(files))
	for _, path := range files {
		file, err := c.operations.ReadFile(ctx, storage, path)
		if err != nil {
			return nil, fmt.Errorf("read metadata file %q: %w", path, err)
		}
		metadata, err := model.NewMetadataFromBytes(file)
		if err != nil {
			return nil, fmt.Errorf("failed to decode backup metadata: %w", err)
		}
		key := keyFromStoragePath(path)
		details := model.NewBackupDetails(*metadata, key, storage)
		backups = append(backups, details)
	}

	return backups, nil
}

// keyFromStoragePath returns the backup key (path to dir containing metadata.yaml, relative to storage).
func keyFromStoragePath(storagePath string) string {
	return filepath.Dir(storagePath)
}

// filterAndSortBackups returns backups whose Created and Finished times fall within bounds sorted chronologically.
func (c *backupCatalog) filterAndSortBackups(
	backups []model.BackupDetails,
	bounds model.TimeBounds,
) []model.BackupDetails {
	out := make([]model.BackupDetails, 0, len(backups))
	for _, b := range backups {
		if bounds.Contains(b.Created) && bounds.Contains(b.Finished) {
			out = append(out, b)
		}
	}

	slices.SortFunc(out, func(a, b model.BackupDetails) int {
		if c := a.Created.Compare(b.Created); c != 0 {
			return c
		}

		return cmp.Compare(a.Key, b.Key)
	})

	return out
}
