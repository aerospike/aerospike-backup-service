package service

import (
	"github.com/aerospike/aerospike-backup-service/pkg/dto"
)

type StorageAccessor interface {
	// readBackupState reads backup state for a backup.
	readBackupState(stateFilePath string, state *dto.BackupState) error

	// readBackupDetails returns backup details for a backup.
	readBackupDetails(path string, useCache bool) (dto.BackupDetails, error)

	// read reads given file.
	read(path string) ([]byte, error)

	// write writes the given byte array to a file
	write(filePath string, v []byte) error

	// lsDir lists all subdirectories in the given path
	// after is an optional filter to return folders after it.
	lsDir(path string, after *string) ([]string, error)

	// lsFiles lists all files in the given path.
	lsFiles(path string) ([]string, error)

	// DeleteFolder removes the folder and all its contents at the specified path.
	DeleteFolder(path string) error

	// wrapWithPrefix combines path with bucket name. This is the opposite of
	// url.parse, required for asbackup library.
	wrapWithPrefix(path string) *string
}
