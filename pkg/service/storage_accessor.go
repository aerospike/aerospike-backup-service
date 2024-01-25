package service

import "github.com/aerospike/backup/pkg/model"

type StorageAccessor interface {
	// readBackupState reads backup state by given file path.
	readBackupState(stateFilePath string, state *model.BackupState) error
	// readBackupDetails read specific backup details by backup folder path.
	readBackupDetails(path string) (model.BackupDetails, error)
	// writeYaml writes given object to file as YAML.
	writeYaml(filePath string, v any) error
	// lists all subdirectories of given path.
	lsDir(path string) ([]string, error)
	// DeleteFolder removes file with a given basePath.
	DeleteFolder(path string) error
	// CreateFolder creates folder with given path.
	CreateFolder(path string)
}
