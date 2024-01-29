package service

import (
	"fmt"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"

	"github.com/aerospike/backup/pkg/model"
)

// BackupBackend handles the backup management logic, employing a StorageAccessor implementation
// for I/O operations.
type BackupBackend struct {
	StorageAccessor
	path                 string
	stateFilePath        string
	removeFiles          bool
	fullBackupInProgress *atomic.Bool // BackupBackend needs to know if full backup is running to filter it out
	stateFileMutex       sync.RWMutex
}

const metadataFile = "metadata.yaml"

func BuildBackupBackends(config *model.Config) map[string]*BackupBackend {
	backends := map[string]*BackupBackend{}
	for routineName := range config.BackupRoutines {
		backends[routineName] = newBackend(config, routineName)
	}
	return backends
}

func newBackend(config *model.Config, routineName string) *BackupBackend {
	backupRoutine := config.BackupRoutines[routineName]
	storage := config.Storage[backupRoutine.Storage]
	backupPolicy := config.BackupPolicies[backupRoutine.BackupPolicy]
	removeFiles := backupPolicy.RemoveFiles != nil && *backupPolicy.RemoveFiles
	switch storage.Type {
	case model.Local:
		path := *storage.Path
		diskAccessor := NewOSDiskAccessor(path)

		return &BackupBackend{
			StorageAccessor:      diskAccessor,
			path:                 path,
			stateFilePath:        path + "/" + model.StateFileName,
			removeFiles:          removeFiles,
			fullBackupInProgress: &atomic.Bool{},
		}
	case model.S3:
		s3Context, err := NewS3Context(storage)
		if err != nil {
			panic(err)
		}

		return &BackupBackend{
			StorageAccessor:      s3Context,
			path:                 s3Context.path,
			stateFilePath:        s3Context.path + "/" + model.StateFileName,
			removeFiles:          removeFiles,
			fullBackupInProgress: &atomic.Bool{},
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

// FullBackupList returns a list of available full backups.
func (b *BackupBackend) FullBackupList(from, to int64) ([]model.BackupDetails, error) {
	backupFolder := b.path + "/" + model.FullBackupDirectory + "/"
	slog.Info("Get full backups", "backupFolder", backupFolder, "from", from, "to", to)

	// when use RemoveFiles = true, backup data is located in backupFolder folder itself
	if b.removeFiles {
		return b.detailsFromPaths(from, to, removeLeadingSlash(backupFolder)), nil
	}

	return b.fromSubfolders(from, to, backupFolder)
}

func (b *BackupBackend) detailsFromPaths(from, to int64, paths ...string) []model.BackupDetails {
	slog.Debug("detailsFromPaths", "from", from, "to", to, "paths", paths)
	backupDetails := []model.BackupDetails{}
	for _, path := range paths {
		namespaces, err := b.lsDir(path)
		if err != nil {
			slog.Error("cannot read backup", "path", path, "err", err)
			return backupDetails
		}
		for _, namespace := range namespaces {
			details, err := b.readBackupDetails(namespace)
			if err != nil {
				slog.Info(err.Error())
				continue
			}
			if details.Created.UnixMilli() >= from &&
				details.Created.UnixMilli() < to {
				backupDetails = append(backupDetails, details)
			}
		}
	}
	return backupDetails
}

func (b *BackupBackend) fromSubfolders(from, to int64, backupFolder string) ([]model.BackupDetails, error) {
	subfolders, err := b.lsDir(backupFolder)
	if err != nil {
		return nil, err
	}

	return b.detailsFromPaths(from, to, subfolders...), nil
}

// IncrementalBackupList returns a list of available incremental backups.
func (b *BackupBackend) IncrementalBackupList() ([]model.BackupDetails, error) {
	backupFolder := b.path + "/" + model.IncrementalBackupDirectory
	return b.fromSubfolders(0, math.MaxInt64, backupFolder)
}

func (b *BackupBackend) FullBackupInProgress() *atomic.Bool {
	return b.fullBackupInProgress
}

func (b *BackupBackend) writeBackupMetadata(path string, metadata model.BackupMetadata) error {
	return b.writeYaml(path+"/"+metadataFile, metadata)
}
