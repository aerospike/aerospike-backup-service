package service

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/service/storage"
	"gopkg.in/yaml.v3"
)

// BackupBackend handles the backup management logic, employing a StorageAccessor
// implementation for I/O operations.
type BackupBackend struct {
	storage                model.Storage
	fullBackupsPath        string
	incrementalBackupsPath string
	stateFilePath          string
	removeFullBackup       bool

	// BackupBackend needs to know if full backup is running to filter it out
	fullBackupInProgress *atomic.Bool
}

var _ BackupListReader = (*BackupBackend)(nil)

func newBackend(routineName string, routine *model.BackupRoutine) *BackupBackend {
	removeFullBackup := routine.BackupPolicy.RemoveFiles.RemoveFullBackup()
	return &BackupBackend{
		storage:                routine.Storage,
		fullBackupsPath:        filepath.Join(routineName, model.FullBackupDirectory),
		incrementalBackupsPath: filepath.Join(routineName, model.IncrementalBackupDirectory),
		stateFilePath:          filepath.Join(routineName, model.StateFileName),
		removeFullBackup:       removeFullBackup,
		fullBackupInProgress:   &atomic.Bool{},
	}
}

func (b *BackupBackend) readState() *model.BackupState {
	to := model.NewTimeBoundsTo(time.Now())
	fullBackupList, _ := b.FullBackupList(context.Background(), to)
	incrementalBackupList, _ := b.IncrementalBackupList(context.Background(), to)

	return &model.BackupState{
		LastFullRun: lastBackupTime(fullBackupList),
		LastIncrRun: lastBackupTime(incrementalBackupList),
	}
}

func lastBackupTime(b []model.BackupDetails) time.Time {
	if len(b) > 0 {
		return latestBackupBeforeTime(b, time.Now())[0].Created
	}

	return time.Time{}
}

func (b *BackupBackend) writeBackupMetadata(ctx context.Context, path string, metadata model.BackupMetadata) error {
	dataYaml, err := yaml.Marshal(metadata)
	if err != nil {
		return err
	}

	metadataFilePath := filepath.Join(path, metadataFile)
	return storage.WriteFile(ctx, b.storage, metadataFilePath, dataYaml)
}

// FullBackupList returns a list of available full backups.
func (b *BackupBackend) FullBackupList(ctx context.Context, timeBounds *model.TimeBounds,
) ([]model.BackupDetails, error) {
	return b.readMetadataList(ctx, timeBounds, true)
}

// IncrementalBackupList returns a list of available incremental backups.
func (b *BackupBackend) IncrementalBackupList(ctx context.Context, timeBounds *model.TimeBounds,
) ([]model.BackupDetails, error) {
	return b.readMetadataList(ctx, timeBounds, false)
}

func (b *BackupBackend) readMetadataList(ctx context.Context, timebounds *model.TimeBounds, isFullBackup bool,
) ([]model.BackupDetails, error) {
	var backupRoot string
	if isFullBackup {
		backupRoot = b.fullBackupsPath
	} else {
		backupRoot = b.incrementalBackupsPath
	}
	files, err := storage.ReadFiles(ctx, b.storage, backupRoot, metadataFile, timebounds.FromTime)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || strings.Contains(err.Error(), "is empty") {
			return nil, nil
		}
		return nil, err
	}

	var backups []model.BackupDetails
	for _, buf := range files {
		metadata, err := model.NewMetadataFromBytes(buf.Bytes())
		if err != nil {
			return nil, fmt.Errorf("error decoding backup metadata YAML: %w", err)
		}
		if timebounds.Contains(metadata.Created) {
			backups = append(backups, model.BackupDetails{
				BackupMetadata: *metadata,
				Key:            getKey(backupRoot, metadata, b.removeFullBackup && isFullBackup),
				Storage:        b.storage,
			})
		}
	}

	return backups, nil
}

// FindLastFullBackup returns last full backup prior to given time.
func (b *BackupBackend) FindLastFullBackup(toTime time.Time) ([]model.BackupDetails, error) {
	fullBackupList, err := b.FullBackupList(context.Background(), model.NewTimeBoundsTo(toTime))
	if err != nil {
		return nil, fmt.Errorf("cannot read full backup list: %w", err)
	}

	fullBackup := latestBackupBeforeTime(fullBackupList, toTime) // it's a list of namespaces
	if len(fullBackup) == 0 {
		return nil, fmt.Errorf("%w: %s", errBackupNotFound, toTime)
	}
	return fullBackup, nil
}

// latestBackupBeforeTime returns list of backups with same creation time,
// latest before upperBound.
func latestBackupBeforeTime(allBackups []model.BackupDetails, upperBound time.Time,
) []model.BackupDetails {
	var result []model.BackupDetails
	var latestTime time.Time
	for i := range allBackups {
		current := &allBackups[i]
		if current.Created.After(upperBound) {
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
	ctx context.Context, bounds *model.TimeBounds, namespace string,
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

func (b *BackupBackend) FullBackupInProgress() *atomic.Bool {
	return b.fullBackupInProgress
}

func (b *BackupBackend) ReadClusterConfiguration(path string) ([]byte, error) {
	configBackups, err := storage.ReadFiles(context.Background(), b.storage, path, configExt, nil)
	if err != nil {
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
