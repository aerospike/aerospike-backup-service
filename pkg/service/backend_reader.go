package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// backupReader holds dependencies and logic for listing and reading backup metadata.
type backupReader struct {
	pathService PathService
	operations  storageOperations
}

func newBackupReader(pathService PathService, operations storageOperations) *backupReader {
	return &backupReader{
		pathService: pathService,
		operations:  operations,
	}
}

func (r *backupReader) getRoutineBackups(ctx context.Context, filter *RoutineFilter) ([]model.BackupDetails, error) {
	backupStorage := filter.routine.Storage

	files, err := r.operations.ReadFileNames(ctx, backupStorage, filter.getPath(), metadataFile, filter.FromTime)
	if err != nil {
		return nil, fmt.Errorf("read metadata files in %s: %w", filter.getPath(), err)
	}

	storagePrefix := filepath.Clean(backupStorage.GetPath())
	files = pathsRelativeToStorage(files, storagePrefix)
	maxPath := filter.getUpperBoundary(r.pathService)
	eligibleFiles := r.filterEligibleFiles(files, maxPath)

	backups, err := r.readBackupDetails(ctx, backupStorage, eligibleFiles)
	if err != nil {
		return nil, err
	}

	backups = r.filterBackups(backups, filter.timeBounds())
	if filter.onlyLast {
		backups = r.getLastBackupsByCreated(backups)
	}

	return backups, nil
}

func (r *backupReader) getPathBackups(ctx context.Context, filter *PathFilter) ([]model.BackupDetails, error) {
	files, err := r.operations.ReadFileNames(ctx, filter.storage, filter.path, metadataFile, nil)
	if err != nil {
		return nil, fmt.Errorf("read metadata files in %s: %w", filter.String(), err)
	}
	files = pathsRelativeToStorage(files, filepath.Clean(filter.storage.GetPath()))
	backups, err := r.readBackupDetails(ctx, filter.storage, files)
	if err != nil {
		return nil, err
	}
	return r.filterBackups(backups, filter.timeBounds()), nil
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
func (r *backupReader) filterEligibleFiles(files []string, maxPath string) []string {
	out := make([]string, 0, len(files))
	for _, p := range files {
		if p < maxPath || strings.HasPrefix(p, maxPath) {
			out = append(out, p)
		}
	}
	return out
}

// readBackupDetails reads and parses metadata for the given metadata file paths.
// Paths must be relative to storage root (same format as ReadFileNames returns).
// Callers must pass only paths for completed backups (metadata exists and Finished is set).
func (r *backupReader) readBackupDetails(
	ctx context.Context,
	storage model.Storage,
	files []string,
) ([]model.BackupDetails, error) {
	backups := make([]model.BackupDetails, 0, len(files))
	for _, path := range files {
		file, err := r.operations.ReadFile(ctx, storage, path)
		if err != nil {
			return nil, fmt.Errorf("read metadata file %q: %w", path, err)
		}
		metadata, err := model.NewMetadataFromBytes(file)
		if err != nil {
			return nil, fmt.Errorf("error decoding backup metadata YAML: %w", err)
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

// filterBackups returns backups whose Created and Finished times fall within bounds.
func (r *backupReader) filterBackups(backups []model.BackupDetails, bounds model.TimeBounds) []model.BackupDetails {
	out := make([]model.BackupDetails, 0, len(backups))
	for _, b := range backups {
		if bounds.Contains(b.Created) && bounds.Contains(b.Finished) {
			out = append(out, b)
		}
	}
	return out
}

// getLastBackupsByCreated returns only backups whose Created time equals the latest.
func (r *backupReader) getLastBackupsByCreated(backups []model.BackupDetails) []model.BackupDetails {
	if len(backups) == 0 {
		return nil
	}

	latest := backups[0].Created
	for _, b := range backups[1:] {
		if b.Created.After(latest) {
			latest = b.Created
		}
	}

	out := make([]model.BackupDetails, 0, len(backups))
	for _, b := range backups {
		if b.Created.Equal(latest) {
			out = append(out, b)
		}
	}

	return out
}
