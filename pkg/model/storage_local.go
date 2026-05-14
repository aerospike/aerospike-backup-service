package model

import "fmt"

// Storage represents the configuration for a backup storage details.
// This interface is implemented by all specific storage types.
type Storage interface {
	// GetPath returns the path to store the backup files.
	GetPath() string
	// GetStorageClass returns the configured storage class of data and metadata.
	GetStorageClass() StorageClass
	// String returns a human-readable representation of the storage.
	String() string
	// GetPartSizeOrDefault returns the size in bytes of individual storage chunks or default value.
	GetPartSizeOrDefault() int
}

// StorageClass defines the storage class of data and metadata.
type StorageClass struct {
	DataClass     string
	MetadataClass string
}

type LocalStorage struct {
	// Path is the root directory where backups will be stored locally.
	Path string
	// MinPartSize is the minimum size in bytes of individual storage chunks.
	MinPartSize *int
}

func (s *LocalStorage) GetStorageClass() StorageClass {
	return StorageClass{}
}

func (s *LocalStorage) GetPath() string {
	return s.Path
}

func (s *LocalStorage) String() string {
	if s.MinPartSize != nil {
		return fmt.Sprintf("LocalStorage(Path: %s, MinPartSize: %d)", s.Path, *s.MinPartSize)
	}

	return fmt.Sprintf("LocalStorage(Path: %s)", s.Path)
}

func (s *LocalStorage) GetPartSizeOrDefault() int {
	if s.MinPartSize != nil {
		return *s.MinPartSize
	}

	return DefaultPartSize
}
