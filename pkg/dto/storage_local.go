package dto

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// LocalStorage represents the configuration for local storage.
// @Description LocalStorage represents the configuration for local storage.
type LocalStorage struct {
	// The root path for the backup repository.
	Path Path `yaml:"path" json:"path" example:"backups" validate:"required"`
	// The minimum size in bytes of individual storage chunks.
	MinPartSize *int `yaml:"min-part-size,omitempty" json:"min-part-size,omitempty" minimum:"1" extensions:"x-nullable"`
}

// Validate checks if the LocalStorage is valid.
func (l *LocalStorage) Validate() error {
	pathStr := string(l.Path)
	if pathStr == "" {
		return errors.New("local storage path is not specified")
	}
	if !filepath.IsAbs(pathStr) && !filepath.IsLocal(pathStr) {
		return errors.New("local storage path must be absolute or local")
	}
	if err := l.Path.Validate(); err != nil {
		return fmt.Errorf("local storage path: %w", err)
	}
	if l.MinPartSize != nil && *l.MinPartSize <= 0 {
		return errors.New("min-part-size for local storage must be a positive value")
	}

	return nil
}

func (l *LocalStorage) toModel() (model.Storage, error) {
	return &model.LocalStorage{
		Path:        string(l.Path),
		MinPartSize: l.MinPartSize,
	}, nil
}

func newLocalStorageFromModel(s *model.LocalStorage) *LocalStorage {
	return &LocalStorage{
		Path:        Path(s.Path),
		MinPartSize: s.MinPartSize,
	}
}
