package model

import "fmt"

// Storage represents the configuration for a backup storage details.
// This interface is implemented by all specific storage types.
type Storage interface {
	GetPath() string
}

type LocalStorage struct {
	// Path is the root directory where backups will be stored locally.
	Path string
}

func (s *LocalStorage) GetPath() string {
	return s.Path
}

func (s *LocalStorage) String() string {
	return fmt.Sprintf("LocalStorage(Path: %s)", s.Path)
}
