package dto

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/safepath"
)

// Storage represents the configuration for a backup storage details.
// @Description Storage represents the configuration for a backup storage details.
type Storage struct {
	// LocalStorage configuration, set if using local storage.
	LocalStorage *LocalStorage `yaml:"local-storage,omitempty" json:"local-storage,omitempty"`
	// S3Storage configuration, set if using S3 storage.
	S3Storage *S3Storage `yaml:"s3-storage,omitempty" json:"s3-storage,omitempty"`
	// GcpStorage configuration, set if using GCP storage.
	GcpStorage *GcpStorage `yaml:"gcp-storage,omitempty" json:"gcp-storage,omitempty"`
	// AzureStorage configuration, set if using Azure storage.
	AzureStorage *AzureStorage `yaml:"azure-storage,omitempty" json:"azure-storage,omitempty"`
}

func validateObjectStoragePath(path string) error {
	if path != "" {
		if !filepath.IsLocal(path) {
			return fmt.Errorf("storage path must be local: %q", path)
		}
		if err := safepath.ValidateClean(path); err != nil {
			return fmt.Errorf("storage path: %w", err)
		}
	}
	return nil
}

// Validate checks if the Storage is valid.
func (s *Storage) Validate(opts ValidationOptions) error {
	if s == nil {
		return errors.New("storage is not specified")
	}

	var validStorage Validator
	count := 0

	if s.LocalStorage != nil {
		validStorage = s.LocalStorage
		count++
	}
	if s.S3Storage != nil {
		validStorage = s.S3Storage
		count++
	}
	if s.GcpStorage != nil {
		validStorage = s.GcpStorage
		count++
	}
	if s.AzureStorage != nil {
		validStorage = s.AzureStorage
		count++
	}
	if count == 0 {
		return errors.New("no storage type specified")
	}
	if count > 1 {
		return fmt.Errorf("multiple storage types specified (%d). Exactly one storage type should be specified", count)
	}

	return validStorage.Validate(opts)
}

// ToModel converts the Storage DTO to its corresponding model.
func (s *Storage) ToModel(c *model.Config) (model.Storage, error) {
	if s.LocalStorage != nil {
		return s.LocalStorage.toModel()
	}
	if s.S3Storage != nil {
		return s.S3Storage.toModel(c)
	}
	if s.GcpStorage != nil {
		return s.GcpStorage.toModel(c)
	}
	if s.AzureStorage != nil {
		return s.AzureStorage.toModel(c)
	}

	return nil, errors.New("error converting storage dto to model: no storage configuration provided")
}

// NewStorageFromModel creates a new Storage DTO from the model.
func NewStorageFromModel(m model.Storage, config *model.BackupConfig) *Storage {
	switch s := m.(type) {
	case *model.LocalStorage:
		return &Storage{
			LocalStorage: newLocalStorageFromModel(s),
		}
	case *model.S3Storage:
		return &Storage{
			S3Storage: newS3StorageFromModel(s, config),
		}
	case *model.GcpStorage:
		return &Storage{
			GcpStorage: newGcpStorageFromModel(s, config),
		}
	case *model.AzureStorage:
		return &Storage{
			AzureStorage: newAzureStorageFromModel(s, config),
		}
	default:
		return nil
	}
}

// NewStorageFromReader creates a new Storage object from a given reader.
func NewStorageFromReader(r io.Reader, format decoder.SerializationFormat) (*Storage, error) {
	s := &Storage{}
	if err := decoder.Deserialize(s, r, format); err != nil {
		return nil, err
	}

	if err := s.Validate(ValidationDefault); err != nil {
		return nil, err
	}

	return s, nil
}
