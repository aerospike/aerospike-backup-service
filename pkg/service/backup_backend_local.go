package service

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"log/slog"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/util"
	"github.com/aws/smithy-go/ptr"
	"gopkg.in/yaml.v3"
)

// BackupBackendLocal implements the BackupBackend interface by
// saving state to the local file system.
type BackupBackendLocal struct {
	path                 string
	stateFilePath        string
	backupPolicy         *model.BackupPolicy
	fullBackupInProgress *atomic.Bool // BackupBackend needs to know if full backup is running to filter it out
	statFileMutex        sync.RWMutex
}

var _ BackupBackend = (*BackupBackendLocal)(nil)

// NewBackupBackendLocal returns a new BackupBackendLocal instance.
func NewBackupBackendLocal(storage *model.Storage, backupPolicy *model.BackupPolicy,
	fullBackupInProgress *atomic.Bool) *BackupBackendLocal {
	path := *storage.Path
	prepareDirectory(path)
	prepareDirectory(path + "/" + model.IncrementalBackupDirectory)
	prepareDirectory(path + "/" + model.FullBackupDirectory)
	return &BackupBackendLocal{
		path:                 path,
		stateFilePath:        path + "/" + model.StateFileName,
		backupPolicy:         backupPolicy,
		fullBackupInProgress: fullBackupInProgress,
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
	local.statFileMutex.RLock()
	defer local.statFileMutex.RUnlock()
	bytes, err := os.ReadFile(local.stateFilePath)
	state := model.NewBackupState()
	if err != nil {
		slog.Warn("Failed to read state file for backup", "err", err)
		return state
	}
	if err = yaml.Unmarshal(bytes, state); err != nil {
		slog.Warn("Failed unmarshal state file for backup", "path",
			local.stateFilePath, "err", err, "content", string(bytes))
	}
	return state
}

func (local *BackupBackendLocal) writeState(state *model.BackupState) error {
	backupState, err := yaml.Marshal(state)
	if err != nil {
		return err
	}
	local.statFileMutex.Lock()
	defer local.statFileMutex.Unlock()
	return os.WriteFile(local.stateFilePath, backupState, 0644)
}

// FullBackupList returns a list of available full backups.
func (local *BackupBackendLocal) FullBackupList() ([]model.BackupDetails, error) {
	backupFolder := local.path + "/" + model.FullBackupDirectory
	entries, err := os.ReadDir(backupFolder)
	if err != nil {
		return nil, err
	}

	lastRun := local.readState().LastRun
	if local.backupPolicy.RemoveFiles != nil && *local.backupPolicy.RemoveFiles {
		// when use RemoveFiles = true, backup data is located in backupFolder folder itself
		if len(entries) == 0 {
			return []model.BackupDetails{}, nil
		}
		if local.fullBackupInProgress.Load() {
			return []model.BackupDetails{}, nil
		}
		return []model.BackupDetails{{
			Key:          ptr.String(backupFolder),
			LastModified: &lastRun,
			Size:         folderSize(backupFolder),
		}}, nil
	}

	backupDetails := make([]model.BackupDetails, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			details := toBackupDetails(e, backupFolder)
			if details.LastModified.Before(lastRun) {
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
		if !e.IsDir() {
			details := toBackupDetails(e, backupFolder)
			if details.LastModified.Before(lastIncrRun) {
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
		if !e.IsDir() {
			filePath := path + "/" + e.Name()
			if err = local.DeleteFile(filePath); err != nil {
				return err
			}
		}
	}
	return nil
}

func (local *BackupBackendLocal) DeleteFile(path string) error {
	err := os.Remove(path)
	if err != nil {
		return fmt.Errorf("failed to delete file: %v", err)
	}
	return nil
}

func toBackupDetails(e fs.DirEntry, prefix string) model.BackupDetails {
	var lastModified *time.Time
	dirInfo, err := e.Info()
	if err == nil {
		lastModified = util.Ptr(dirInfo.ModTime())
	}
	path := filepath.Join(prefix, e.Name())
	return model.BackupDetails{
		Key:          util.Ptr(path),
		LastModified: lastModified,
		Size:         folderSize(path),
	}
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

	slog.Debug("Calculated size", "path", path, "size", size)
	return &size
}
