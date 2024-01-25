package service

import (
	"errors"
	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/util"
	"gopkg.in/yaml.v3"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// OSDiskAccessor is responsible for IO operation on local disk.
type OSDiskAccessor struct {
	basePath string
}

func NewOS(path string) OSDiskAccessor {
	return OSDiskAccessor{basePath: path}
}

func (_ *OSDiskAccessor) readBackupState(filepath string, state *model.BackupState) error {
	bytes, err := os.ReadFile(filepath)
	if err != nil {
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) &&
			strings.Contains(pathErr.Error(), "no such file or directory") {
			slog.Debug("State file does not exist for backup", "path", filepath,
				"err", err)
			return nil
		}
		slog.Warn("Failed to read state file for backup", "path", filepath,
			"err", err)
		return err
	}
	if err = yaml.Unmarshal(bytes, state); err != nil {
		slog.Warn("Failed unmarshal state file for backup", "path", filepath,
			"err", err, "content", string(bytes))
	}
	return nil
}

func (_ *OSDiskAccessor) readBackupDetails(path string) (model.BackupDetails, error) {
	metadata := &model.BackupMetadata{}
	filePath := filepath.Join(path, metadataFile)
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return model.BackupDetails{}, err
	}

	if err = yaml.Unmarshal(bytes, metadata); err != nil {
		slog.Warn("Failed unmarshal metadata file", "basePath",
			filePath, "err", err, "content", string(bytes))
		return model.BackupDetails{}, err
	}

	if err != nil {
		return model.BackupDetails{}, err
	}
	return model.BackupDetails{
		BackupMetadata: *metadata,
		Key:            util.Ptr(path),
	}, nil
}

func (_ *OSDiskAccessor) writeYaml(filePath string, v any) error {
	backupState, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, backupState, 0644)
}

func (_ *OSDiskAccessor) lsDir(path string) ([]string, error) {
	content, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var onlyDirs []string
	for _, c := range content {
		if c.IsDir() {
			fullPath := filepath.Join(path, c.Name())
			onlyDirs = append(onlyDirs, fullPath)
		}
	}
	return onlyDirs, nil
}
func (_ *OSDiskAccessor) CreateFolder(path string) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		if err = os.MkdirAll(path, 0744); err != nil {
			slog.Warn("Error creating backup directory", "basePath", path, "err", err)
		}
	}
	if err = os.Chmod(path, 0744); err != nil {
		slog.Warn("Failed to Chmod backup directory", "basePath", path, "err", err)
	}
}

func (o *OSDiskAccessor) DeleteFolder(pathToDelete string) error {
	slog.Info("Delete all " + pathToDelete)
	return os.RemoveAll(filepath.Join(o.basePath, pathToDelete))
}
