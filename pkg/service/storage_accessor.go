package service

import (
	"github.com/aerospike/backup/pkg/model"
	"time"
)

type StorageAccessor interface {
	// readBackupState reads backup state for a backup.
	readBackupState(stateFilePath string, state *model.BackupState) error
	// readBackupDetails returns backup details for a backup.
	readBackupDetails(path string, useCache bool) (model.BackupDetails, error)
	// read reads given file.
	read(path string) ([]byte, error)
	// write writes the given byte array to a file
	write(filePath string, v []byte) error
	// lsDir lists all subdirectories in the given path.
	lsDir(path string) ([]string, error)
	// lsFiles lists all files in the given path.
	lsFiles(path string) ([]string, error)
	// DeleteFolder removes the folder and all its contents at the specified path.
	DeleteFolder(path string) error
	// wrapWithPrefix combines path with bucket name. This is the opposite of url.parse, required for asbackup library.
	wrapWithPrefix(path string) *string

	lsDirAfter(prefix string, after *time.Time) ([]string, error)
}
