package model

import "fmt"

// Storage represents the configuration for a backup storage details.
// This interface is implemented by all specific storage types.
type Storage interface {
	storage()
}

type LocalStorage struct {
	// Path is the root directory where backups will be stored locally.
	Path string
}

func (s *LocalStorage) storage() {}
func (s *LocalStorage) String() string {
	return fmt.Sprintf("LocalStorage(Path: %s)", s.Path)
}
