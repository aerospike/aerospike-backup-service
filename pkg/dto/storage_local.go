package dto

import (
	"errors"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// LocalStorage represents the configuration for local storage.
type LocalStorage struct {
	// The root path for the backup repository.
	Path string `yaml:"path" json:"path" example:"backups" validate:"required"`
}

// Validate checks if the LocalStorage is valid.
func (l *LocalStorage) Validate() error {
	if l.Path == "" {
		return errors.New("local storage path is not specified")
	}
	return nil
}

func (l *LocalStorage) toModel() (model.Storage, error) {
	return &model.LocalStorage{
		Path: l.Path,
	}, nil
}

func newLocalStorageFromModel(s *model.LocalStorage) *LocalStorage {
	return &LocalStorage{
		Path: s.Path,
	}
}
