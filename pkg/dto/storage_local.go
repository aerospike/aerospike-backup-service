package dto

import (
	"errors"

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
	// Local filesystem storage roots may be absolute or relative, unlike
	// object-storage prefixes and backup-data-path.
	if err := l.Path.Validate(ValidationAllowAbsolutePath); err != nil {
		return errValidationInvalidPath("path", l.Path, err)
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
