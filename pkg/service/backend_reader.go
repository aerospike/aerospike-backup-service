package service

import (
	"cmp"
	"context"
	"fmt"
	"path/filepath"
	"slices"
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
	backupStorage := filter.storage

	files, err := r.operations.ReadFileNames(ctx, backupStorage, filter.getPath(), metadataFile, filter.FromTime)
	if err != nil {
		return nil, fmt.Errorf("read metadata files in %s: %w", filter.getPath(), err)
	}

	storagePrefix := filepath.Clean(backupStorage.GetPath())
	files = pathsRelativeToStorage(files, storagePrefix)
	maxPath := filter.getUpperBoundary(r.pathService)
	eligibleFiles := r.filterEligibleFiles(files, maxPath)

	if filter.onlyLast {
		return r.readLatestBackupDetails(ctx, backupStorage, filter, eligibleFiles)
	}

	backups, err := r.readBackupDetails(ctx, backupStorage, eligibleFiles)
	if err != nil {
		return nil, err
	}

	backups = r.filterAndSortBackups(backups, filter.timeBounds())

	return backups, nil
}

func (r *backupReader) readLatestBackupDetails(
	ctx context.Context,
	storage model.Storage,
	filter *RoutineFilter,
	files []string,
) ([]model.BackupDetails, error) {
	bounds := filter.timeBounds()
	pathsByTimestamp := r.groupMetadataPathsByTimestamp(files)

	for len(pathsByTimestamp) > 0 {
		// The storage layout is routine/backup/<timestamp>/data/<namespace>/metadata.yaml,
		// so the newest timestamp directory identifies the last backup run.
		timestamp := maxTimestamp(pathsByTimestamp)
		paths := pathsByTimestamp[timestamp]
		slices.Sort(paths)

		backups, err := r.readBackupDetails(ctx, storage, paths)
		if err != nil {
			return nil, err
		}

		// Created is represented by the timestamp path, but Finished only exists
		// in metadata. If this timestamp does not match the full bounds, try the
		// next newest timestamp.
		backups = r.filterAndSortBackups(backups, bounds)
		if len(backups) > 0 {
			return backups, nil
		}

		delete(pathsByTimestamp, timestamp)
	}

	return nil, nil
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
	return r.filterAndSortBackups(backups, filter.timeBounds()), nil
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

func (r *backupReader) groupMetadataPathsByTimestamp(files []string) map[string][]string {
	pathsByTimestamp := make(map[string][]string)
	for _, file := range files {
		timestamp := r.pathService.ExtractTimestampFromPath(file)
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
func (r *backupReader) filterAndSortBackups(
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
