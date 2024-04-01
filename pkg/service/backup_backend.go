package service

import (
	"archive/zip"
	"bytes"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/aerospike/backup/pkg/model"
	"gopkg.in/yaml.v3"
)

// BackupBackend handles the backup management logic, employing a StorageAccessor implementation
// for I/O operations.
type BackupBackend struct {
	StorageAccessor
	fullBackupsPath        string
	incrementalBackupsPath string
	stateFilePath          string
	removeFullBackup       bool
	fullBackupInProgress   *atomic.Bool // BackupBackend needs to know if full backup is running to filter it out
	stateFileMutex         sync.RWMutex
}

var _ BackupListReader = (*BackupBackend)(nil)

const metadataFile = "metadata.yaml"

func newBackend(config *model.Config, routineName string) *BackupBackend {
	backupRoutine := config.BackupRoutines[routineName]
	storage := config.Storage[backupRoutine.Storage]
	backupPolicy := config.BackupPolicies[backupRoutine.BackupPolicy]
	removeFullBackup := backupPolicy.RemoveFiles.RemoveFullBackup()
	switch storage.Type {
	case model.Local:
		routinePath := filepath.Join(*storage.Path, routineName)
		return &BackupBackend{
			StorageAccessor:        NewOSDiskAccessor(),
			fullBackupsPath:        filepath.Join(routinePath, model.FullBackupDirectory),
			incrementalBackupsPath: filepath.Join(routinePath, model.IncrementalBackupDirectory),
			stateFilePath:          filepath.Join(routinePath, model.StateFileName),
			removeFullBackup:       removeFullBackup,
			fullBackupInProgress:   &atomic.Bool{},
		}
	case model.S3:
		s3Context, err := NewS3Context(storage)
		if err != nil {
			panic(err)
		}

		routinePath := filepath.Join(s3Context.path, routineName)
		return &BackupBackend{
			StorageAccessor:        s3Context,
			fullBackupsPath:        filepath.Join(routinePath, model.FullBackupDirectory),
			incrementalBackupsPath: filepath.Join(routinePath, model.IncrementalBackupDirectory),
			stateFilePath:          filepath.Join(routinePath, model.StateFileName),
			removeFullBackup:       removeFullBackup,
			fullBackupInProgress:   &atomic.Bool{},
		}
	default:
		panic(fmt.Sprintf("Unsupported storage type: %v", storage.Type))
	}
}

func (b *BackupBackend) readState() *model.BackupState {
	b.stateFileMutex.RLock()
	defer b.stateFileMutex.RUnlock()
	state := model.NewBackupState()
	err := b.readBackupState(b.stateFilePath, state)
	if err != nil {
		slog.Warn("Failed to read backup state", "path", b.stateFilePath, "err", err)
	}
	return state
}

func (b *BackupBackend) writeState(state *model.BackupState) error {
	b.stateFileMutex.Lock()
	defer b.stateFileMutex.Unlock()
	return b.writeYaml(b.stateFilePath, state)
}

func (b *BackupBackend) writeBackupMetadata(path string, metadata model.BackupMetadata) error {
	metadataFilePath := filepath.Join(path, metadataFile)
	return b.writeYaml(metadataFilePath, metadata)
}

func (b *BackupBackend) writeYaml(path string, data any) error {
	dataYaml, err := yaml.Marshal(data)
	if err != nil {
		return err
	}

	return b.write(path, dataYaml)
}

// FullBackupList returns a list of available full backups.
func (b *BackupBackend) FullBackupList(timebounds *model.TimeBounds) ([]model.BackupDetails, error) {
	slog.Info("Get full backups", "backupFolder", b.fullBackupsPath,
		"timebounds", timebounds, "removeFullBackup", b.removeFullBackup)

	// when use RemoveFiles.RemoveFullBackup() = true, backup data is located in backupFolder folder itself
	if b.removeFullBackup {
		return b.detailsFromPaths(timebounds, false, b.fullBackupsPath), nil
	}

	return b.fromSubfolders(timebounds, b.fullBackupsPath)
}

func (b *BackupBackend) detailsFromPaths(timebounds *model.TimeBounds, useCache bool,
	paths ...string) []model.BackupDetails {
	// each path contains a backup of specific time
	backupDetails := []model.BackupDetails{}
	for _, path := range paths {
		namespaces, err := b.lsDir(filepath.Join(path, model.DataDirectory))
		if err != nil {
			slog.Warn("Cannot list backup dir", "path", path, "err", err)
			continue
		}
		for _, namespacePath := range namespaces {
			details, err := b.readBackupDetails(namespacePath, useCache)
			if err != nil {
				slog.Debug("Cannot read backup details", "err", err)
				continue
			}
			if timebounds.Contains(details.Created.UnixMilli()) {
				backupDetails = append(backupDetails, details)
			}
		}
	}
	return backupDetails
}

func (b *BackupBackend) fromSubfolders(timebounds *model.TimeBounds,
	backupFolder string) ([]model.BackupDetails, error) {
	subfolders, err := b.lsDir(backupFolder)
	if err != nil {
		return nil, err
	}
	return b.detailsFromPaths(timebounds, true, subfolders...), nil
}

// IncrementalBackupList returns a list of available incremental backups.
func (b *BackupBackend) IncrementalBackupList(timebounds *model.TimeBounds) ([]model.BackupDetails, error) {
	return b.fromSubfolders(timebounds, b.incrementalBackupsPath)
}

func (b *BackupBackend) FullBackupInProgress() *atomic.Bool {
	return b.fullBackupInProgress
}

func (b *BackupBackend) ReadClusterConfiguration(path string) ([]byte, error) {
	configBackups, err := b.lsFiles(path)
	if err != nil {
		return nil, err
	}

	return b.packageFiles(configBackups)
}

// PackageFiles creates a zip archive from the given file list and returns it as a byte array
func (b *BackupBackend) packageFiles(files []string) ([]byte, error) {
	// Create a buffer to write our archive to
	buf := new(bytes.Buffer)

	// Create a new zip archive
	w := zip.NewWriter(buf)

	for _, file := range files {
		data, err := b.read(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", file, err)
		}

		f, err := w.Create(filepath.Base(file))
		if err != nil {
			return nil, fmt.Errorf("failed to create entry for filename %s: %w", file, err)
		}

		_, err = f.Write(data)
		if err != nil {
			return nil, fmt.Errorf("failed to write file %s: %w", file, err)
		}
	}

	err := w.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close the zip writer: %w", err)
	}

	return buf.Bytes(), nil
}
