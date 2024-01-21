package service

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"log/slog"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/util"
	"gopkg.in/yaml.v3"
)

// BackupBackendLocal implements the BackupBackend interface by
// saving state to the local file system.
type BackupBackendLocal struct {
	path                 string
	stateFilePath        string
	backupPolicy         *model.BackupPolicy
	fullBackupInProgress *atomic.Bool // BackupBackend needs to know if full backup is running to filter it out
	stateFileMutex       sync.RWMutex
}

var _ BackupBackend = (*BackupBackendLocal)(nil)

const metadataFile = "metadata.yaml"

// NewBackupBackendLocal returns a new BackupBackendLocal instance.
func NewBackupBackendLocal(storage *model.Storage, backupPolicy *model.BackupPolicy) *BackupBackendLocal {
	path := *storage.Path
	prepareDirectory(path)
	prepareDirectory(path + "/" + model.IncrementalBackupDirectory)
	prepareDirectory(path + "/" + model.FullBackupDirectory)
	return &BackupBackendLocal{
		path:                 path,
		stateFilePath:        path + "/" + model.StateFileName,
		backupPolicy:         backupPolicy,
		fullBackupInProgress: &atomic.Bool{},
	}
}

func prepareDirectory(path string) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		if err = os.Mkdir(path, 0744); err != nil {
			slog.Warn("Error creating backup directory", "path", path, "err", err)
		}
	}
	if err = os.Chmod(path, 0744); err != nil {
		slog.Warn("Failed to Chmod backup directory", "path", path, "err", err)
	}
}

func (local *BackupBackendLocal) readState() *model.BackupState {
	local.stateFileMutex.RLock()
	bytes, err := os.ReadFile(local.stateFilePath)
	local.stateFileMutex.RUnlock()

	state := model.NewBackupState()
	if err != nil {
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) &&
			strings.Contains(pathErr.Error(), "no such file or directory") {
			slog.Debug("State file does not exist for backup", "path", local.stateFilePath,
				"err", err)
		} else {
			slog.Warn("Failed to read state file for backup", "path", local.stateFilePath,
				"err", err)
		}
		return state
	}
	if err = yaml.Unmarshal(bytes, state); err != nil {
		slog.Warn("Failed unmarshal state file for backup", "path", local.stateFilePath,
			"err", err, "content", string(bytes))
	}
	return state
}

func (local *BackupBackendLocal) writeState(state *model.BackupState) error {
	backupState, err := yaml.Marshal(state)
	if err != nil {
		return err
	}
	local.stateFileMutex.Lock()
	defer local.stateFileMutex.Unlock()
	return os.WriteFile(local.stateFilePath, backupState, 0644)
}

// FullBackupList returns a list of available full backups.
func (local *BackupBackendLocal) FullBackupList(from, to int64) ([]model.BackupDetails, error) {
	backupFolder := local.path + "/" + model.FullBackupDirectory
	lastRun := local.readState().LastFullRun
	slog.Info("get full backups", "backupFolder", backupFolder, "lastRun", lastRun, "from", from, "to", to)

	entries, err := os.ReadDir(backupFolder)
	if err != nil {
		return nil, err
	}

	if local.backupPolicy.RemoveFiles != nil && *local.backupPolicy.RemoveFiles {
		// when use RemoveFiles = true, backup data is located in backupFolder folder itself
		if len(entries) == 0 {
			return []model.BackupDetails{}, nil
		}
		if local.fullBackupInProgress.Load() {
			return []model.BackupDetails{}, nil
		}
		// check request time boundaries
		if lastRun.UnixMilli() < from || lastRun.UnixMilli() >= to {
			return []model.BackupDetails{}, nil
		}
		details, err := local.toBackupDetails(backupFolder)
		if err != nil {
			return nil, err
		}
		return []model.BackupDetails{details}, nil
	}

	backupDetails := make([]model.BackupDetails, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			path := filepath.Join(backupFolder, e.Name())
			details, err := local.toBackupDetails(path)
			if err != nil { // no backup metadata file
				continue
			}
			if details.Created.UnixMilli() >= from &&
				details.Created.UnixMilli() < to {
				backupDetails = append(backupDetails, details)
			}
		}
	}
	return backupDetails, nil
}

// IncrementalBackupList returns a list of available incremental backups.
func (local *BackupBackendLocal) IncrementalBackupList() ([]model.BackupDetails, error) {
	backupFolder := local.path + "/" + model.IncrementalBackupDirectory
	entries, err := os.ReadDir(backupFolder)
	if err != nil {
		return nil, err
	}
	lastIncrRun := local.readState().LastIncrRun
	backupDetails := make([]model.BackupDetails, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			path := filepath.Join(backupFolder, e.Name())
			details, err := local.toBackupDetails(path)
			if err != nil { // no backup metadata file
				continue
			}
			if !details.Created.After(lastIncrRun) {
				backupDetails = append(backupDetails, details)
			}
		}
	}
	return backupDetails, nil
}

// CleanDir cleans the directory with the given name.
func (local *BackupBackendLocal) CleanDir(name string) error {
	path := fmt.Sprintf("%s/%s/", local.path, name)
	dir, err := os.ReadDir(path)
	if err != nil {
		slog.Warn("Failed to read directory", "path", path, "err", err)
	}
	for _, e := range dir {
		filePath := path + "/" + e.Name()
		if err = local.DeleteFolder(filePath); err != nil {
			return err
		}
	}
	return nil
}

func (local *BackupBackendLocal) DeleteFolder(path string) error {
	return os.RemoveAll(path)
}

func (local *BackupBackendLocal) toBackupDetails(path string) (model.BackupDetails, error) {
	metadata, err := local.readBackupMetadata(path)
	if err != nil {
		return model.BackupDetails{}, err
	}
	return model.BackupDetails{
		BackupMetadata: *metadata,
		Key:            util.Ptr(path),
	}, nil
}

func folderSize(path string) *int64 {
	var size int64

	err := filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			size += info.Size()
		}
		return nil
	})

	if err != nil {
		slog.Error("failed to calculate size", "path", path, "err", err)
		return nil
	}

	return &size
}

func (local *BackupBackendLocal) writeBackupMetadata(path string, metadata model.BackupMetadata) error {
	metadataBytes, err := yaml.Marshal(metadata)
	if err != nil {
		return err
	}
	return os.WriteFile(path+"/"+metadataFile, metadataBytes, 0644)
}

func (local *BackupBackendLocal) readBackupMetadata(path string) (*model.BackupMetadata, error) {
	metadata := &model.BackupMetadata{}
	filePath := path + "/" + metadataFile
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return metadata, err
	}

	if err = yaml.Unmarshal(bytes, metadata); err != nil {
		slog.Warn("Failed unmarshal metadata file", "path",
			filePath, "err", err, "content", string(bytes))
		return metadata, err
	}

	return metadata, nil
}

func (local *BackupBackendLocal) FullBackupInProgress() *atomic.Bool {
	return local.fullBackupInProgress
}
