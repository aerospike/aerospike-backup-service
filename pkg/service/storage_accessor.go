package service

import "github.com/aerospike/backup/pkg/model"

type StorageAccessor interface {
	// readBackupState reads backup state for a backup.
	readBackupState(stateFilePath string, state *model.BackupState) error
	// readBackupDetails returns backup details for a backup.
	readBackupDetails(path string, useCache bool) (model.BackupDetails, error)
	// writeYaml writes the given object to a file in the YAML format.
	writeYaml(filePath string, v any) error
	// lsDir lists all subdirectories in the given path.
	lsDir(path string) ([]string, error)
	// DeleteFolder removes the folder and all its contents at the specified path.
	DeleteFolder(path string) error
	// CreateFolder creates a folder at the specified path.
	CreateFolder(path string)
	// wrapWithPrefix combines path with bucket name. This is the opposite of url.parse, required for asbackup library.
	wrapWithPrefix(path string) *string
}
