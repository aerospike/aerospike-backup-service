package service

import (
	"encoding/json"
	"os"

	"log/slog"

	"github.com/aerospike/backup/pkg/model"
)

// BackupBackendLocal implements the BackupBackend interface by
// saving state to the local file system.
type BackupBackendLocal struct {
	path          string
	stateFilePath string
}

var _ BackupBackend = (*BackupBackendLocal)(nil)

// NewBackupBackendLocal returns a new BackupBackendLocal instance.
func NewBackupBackendLocal(path string) *BackupBackendLocal {
	return &BackupBackendLocal{
		path:          path,
		stateFilePath: path + "/" + stateFileName,
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
		slog.Warn("Failed unmarshal state file for backup", "path", local.stateFilePath, "err", err)
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

func (local *BackupBackendLocal) backupList() ([]string, error) {
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
