package service

import (
	"encoding/json"
	"fmt"
	"os"

	"log/slog"

	"github.com/aerospike/backup/pkg/model"
)

// BackupBackendLocal implements the BackupBackend interface by
// saving state to the local file system.
type BackupBackendLocal struct {
	path             string
	stateFilePath    string
	backupPolicyName string
}

var _ BackupBackend = (*BackupBackendLocal)(nil)

// NewBackupBackendLocal returns a new BackupBackendLocal instance.
func NewBackupBackendLocal(path, backupPolicyName string) *BackupBackendLocal {
	if err := os.Chmod(path, 0744); err != nil {
		slog.Warn("Failed to Chmod backup directory", "path", path, "err", err)
	}
	incrDirectoryPath := path + "/" + incremenalBackupDirectory
	if err := os.Mkdir(incrDirectoryPath, 0744); err != nil {
		slog.Debug("Failed to Mkdir incremental backup directory",
			"path", incrDirectoryPath, "err", err)
	}
	return &BackupBackendLocal{
		path:             path,
		stateFilePath:    path + "/" + stateFileName,
		backupPolicyName: backupPolicyName,
	}
}

func (local *BackupBackendLocal) readState() *model.BackupState {
	bytes, err := os.ReadFile(local.stateFilePath)
	state := model.NewBackupState()
	if err != nil {
		slog.Warn("Failed to read state file for backup", "err", err)
		return state
	}
	if err = json.Unmarshal(bytes, state); err != nil {
		slog.Warn("Failed unmarshal state file for backup", "path",
			local.stateFilePath, "err", err)
	}
	return state
}

func (local *BackupBackendLocal) writeState(state *model.BackupState) error {
	backupState, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(local.stateFilePath, backupState, 0644)
}

func (local *BackupBackendLocal) FullBackupList() ([]string, error) {
	entries, err := os.ReadDir(local.path)
	if err != nil {
		return nil, err
	}

	var backupFolders []string
	for _, e := range entries {
		if e.IsDir() {
			backupFolders = append(backupFolders, e.Name())
		}
	}
	return backupFolders, nil
}

func (local *BackupBackendLocal) IncrementalBackupList() ([]string, error) {
	entries, err := os.ReadDir(local.path + "/" + incremenalBackupDirectory)
	if err != nil {
		return nil, err
	}

	var backupFiles []string
	for _, e := range entries {
		if !e.IsDir() {
			backupFiles = append(backupFiles, e.Name())
		}
	}
	return backupFiles, nil
}

func (local *BackupBackendLocal) CleanDir(name string) {
	path := fmt.Sprintf("%s/%s/", local.path, name)
	dir, err := os.ReadDir(path)
	if err != nil {
		slog.Warn("Failed to read directory", "path", path, "err", err)
	}
	for _, e := range dir {
		if !e.IsDir() {
			filePath := path + "/" + e.Name()
			if err = os.Remove(filePath); err != nil {
				slog.Debug("Failed to delete file", "path", filePath, "err", err)
			}
		}
	}
}

func (local *BackupBackendLocal) BackupPolicyName() string {
	return local.backupPolicyName
}
