package service

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	"io"
	"log/slog"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"gopkg.in/yaml.v3"
)

// BackupBackend handles the backup management logic, employing a StorageAccessor
// implementation for I/O operations.
type BackupBackend struct {
	mu          sync.RWMutex
	routineName string
	routine     *model.BackupRoutine
}

var _ BackupMetadataReaderWriter = (*BackupBackend)(nil)

func newBackend(routineName string, routine *model.BackupRoutine) *BackupBackend {
	return &BackupBackend{
		routineName: routineName,
		routine:     routine,
	}
}

func (b *BackupBackend) writeBackupMetadata(ctx context.Context, path string, metadata model.BackupMetadata) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	dataYaml, err := yaml.Marshal(metadata)
	if err != nil {
		return err
	}

	metadataFilePath := filepath.Join(path, metadataFile)
	return storage.WriteMetadataFile(ctx, b.routine.Storage, metadataFilePath, dataYaml)
}

// FullBackupList returns a list of available full backups.
func (b *BackupBackend) FullBackupList(ctx context.Context, timeBounds model.TimeBounds,
) ([]model.BackupDetails, error) {
	return b.readMetadataList(ctx, timeBounds, jobTypeFull)
}

// IncrementalBackupList returns a list of available incremental backups.
func (b *BackupBackend) IncrementalBackupList(ctx context.Context, timeBounds model.TimeBounds,
) ([]model.BackupDetails, error) {
	return b.readMetadataList(ctx, timeBounds, jobTypeIncremental)
}

func (b *BackupBackend) readMetadataList(
	ctx context.Context, timeBounds model.TimeBounds, backupType jobType,
) ([]model.BackupDetails, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	backupRoot := getBackupRootPath(b.routineName, backupType)
	files, err := storage.ReadFiles(ctx, b.routine.Storage, backupRoot, metadataFile, timeBounds.FromTime)
	if err != nil {
		if errors.Is(err, storage.ErrEmptyStorage) {
			slog.Info("Read metadata files empty",
				slog.String("path", backupRoot),
				slog.Any("storage", b.routine.Storage),
				slog.Any("timebounds", timeBounds.String()))
			return nil, nil
		}
		return nil, fmt.Errorf("read metadata files error: %w", err)
	}

	slog.Info("Read metadata files",
		slog.Int("files count", len(files)),
		slog.String("path", backupRoot),
		slog.Any("storage", b.routine.Storage))

	var backups []model.BackupDetails
	for _, buf := range files {
		metadata, err := model.NewMetadataFromBytes(buf.Bytes())
		if err != nil {
			return nil, fmt.Errorf("error decoding backup metadata YAML: %w", err)
		}
		if timeBounds.Contains(metadata.Created) {
			backups = append(backups, model.BackupDetails{
				BackupMetadata: *metadata,
				Key:            getKey(b.routineName, backupType, metadata),
				Storage:        b.routine.Storage,
			})
		}
	}

	return backups, nil
}

const maxRequests = 5

// FindLastFullBackup returns last full backup prior to given time.
// returns error when not found.
func (b *BackupBackend) FindLastFullBackup(ctx context.Context, toTime *time.Time) ([]model.BackupDetails, error) {
	var fromTime = time.Now()
	if toTime != nil {
		fromTime = *toTime
	}

	prevSchedule := util.PreviousCron(fromTime, b.routine.IntervalCron)
	duration := fromTime.Sub(prevSchedule) // start with difference between previous schedule and now

	// Start with an small range and double it until we find a backup or exceed a maximum range.
	for range maxRequests {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		var fromTime = time.Now().Add(-duration)
		if toTime != nil {
			fromTime = toTime.Add(-duration)
		}

		if fromTime.Before(time.Unix(0, 0)) {
			break
		}
		timeBounds, _ := model.NewTimeBounds(&fromTime, toTime)
		slog.Info("Searching for full backup",
			slog.Duration("duration", duration),
			slog.String("timebounds", timeBounds.String()),
			slog.Any("routine", b.routineName))

		fullBackupList, err := b.FullBackupList(ctx, timeBounds)
		if err != nil {
			return nil, fmt.Errorf("cannot read full backup list: %w", err)
		}
		fullBackup := latestBackupBeforeTime(fullBackupList, toTime)
		if len(fullBackup) > 0 {
			return fullBackup, nil
		}

		duration *= 2
	}

	// If no backup was found, make a final attempt without any bounds
	fullBackupList, err := b.FullBackupList(ctx, model.TimeBounds{ToTime: toTime})
	if err != nil {
		return nil, fmt.Errorf("cannot read full backup list: %w", err)
	}

	fullBackup := latestBackupBeforeTime(fullBackupList, toTime)
	if len(fullBackup) == 0 {
		return nil, fmt.Errorf("%w before time %s", errBackupNotFound, toTime)
	}
	return fullBackup, nil
}

// latestBackupBeforeTime returns list of backups with same creation time,
// latest before upperBound.
func latestBackupBeforeTime(allBackups []model.BackupDetails, upperBound *time.Time,
) []model.BackupDetails {
	var result []model.BackupDetails
	var latestTime time.Time
	for i := range allBackups {
		current := &allBackups[i]
		if upperBound != nil && current.Created.After(*upperBound) {
			continue
		}

		if len(result) == 0 || latestTime.Before(current.Created) {
			latestTime = current.Created
			result = []model.BackupDetails{*current}
		} else if current.Created.Equal(latestTime) {
			result = append(result, *current)
		}
	}

	return result
}

// FindIncrementalBackupsForNamespace returns all incremental backups in given range, sorted by time.
func (b *BackupBackend) FindIncrementalBackupsForNamespace(
	ctx context.Context, bounds model.TimeBounds, namespace string,
) ([]model.BackupDetails, error) {
	allIncrementalBackupList, err := b.IncrementalBackupList(ctx, bounds)
	if err != nil {
		return nil, err
	}

	var filteredIncrementalBackups []model.BackupDetails
	for _, b := range allIncrementalBackupList {
		if b.Namespace == namespace {
			filteredIncrementalBackups = append(filteredIncrementalBackups, b)
		}
	}
	// Sort in place
	sort.Slice(filteredIncrementalBackups, func(i, j int) bool {
		return filteredIncrementalBackups[i].Created.Before(filteredIncrementalBackups[j].Created)
	})

	return filteredIncrementalBackups, nil
}

func (b *BackupBackend) ReadClusterConfiguration(ctx context.Context, path string) ([]byte, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	configBackups, err := storage.ReadFiles(ctx, b.routine.Storage, path, configExt, nil)
	if err != nil && !errors.Is(err, storage.ErrEmptyStorage) {
		return nil, err
	}
	if len(configBackups) == 0 {
		return nil, fmt.Errorf("no configuration backups found for %s", path)
	}

	return b.packageFiles(configBackups)
}

// packageFiles creates a zip archive from the given file list and returns it as a byte array.
func (b *BackupBackend) packageFiles(buffers []*bytes.Buffer) ([]byte, error) {
	// Create a buffer to write our archive to
	buf := new(bytes.Buffer)

	// Create a new zip archive
	w := zip.NewWriter(buf)

	for i, data := range buffers {
		fileName := getConfigFileName(i)

		f, err := w.Create(fileName)
		if err != nil {
			return nil, fmt.Errorf("failed to create entry for filename %s: %w", fileName, err)
		}

		_, err = io.Copy(f, data)
		if err != nil {
			return nil, fmt.Errorf("failed to write buffer %d: %w", i, err)
		}
	}

	err := w.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close the zip writer: %w", err)
	}

	return buf.Bytes(), nil
}

func (b *BackupBackend) deleteFolder(ctx context.Context, path string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	return storage.DeleteFolder(ctx, b.routine.Storage, path)
}
