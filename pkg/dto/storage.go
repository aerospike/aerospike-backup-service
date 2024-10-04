package dto

import (
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
)

// Storage represents the configuration for a backup storage details.
// @Description Storage represents the configuration for a backup storage details.
type Storage struct {
	// LocalStorage configuration, set if using local storage
	LocalStorage *LocalStorage `yaml:"local-storage,omitempty" json:"local-storage,omitempty"`
	// S3Storage configuration, set if using S3 storage
	S3Storage *S3Storage `yaml:"s3-storage,omitempty" json:"s3-storage,omitempty"`
	// GcpStorage configuration, set if using GCP storage
	GcpStorage *GcpStorage `yaml:"gcp-storage,omitempty" json:"gcp-storage,omitempty"`
}

// StorageValidator interface for storage types that can be validated
type StorageValidator interface {
	Validate() error
}

// Validate checks if the Storage is valid.
func (s *Storage) Validate() error {
	if s == nil {
		return errors.New("storage is not specified")
	}

	var validStorage StorageValidator
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
	if count == 0 {
		return errors.New("no storage type specified")
	}
	if count > 1 {
		return fmt.Errorf("multiple storage types specified (%d). Exactly one storage type should be specified", count)
	}

	return validStorage.Validate()
}

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

// S3Storage represents the configuration for S3 storage.
//
//nolint:lll
type S3Storage struct {
	// The S3 bucket name.
	Bucket string `yaml:"bucket" json:"bucket" validate:"required"`
	// The root path for the backup repository within the bucket.
	Path string `yaml:"path" json:"path" example:"backups" validate:"required"`
	// The S3 region string.
	S3Region string `yaml:"s3-region" json:"s3-region" example:"eu-central-1" validate:"required"`
	// The S3 profile name (AWS S3 optional).
	S3Profile string `yaml:"s3-profile,omitempty" json:"s3-profile,omitempty" example:"default"`
	// An alternative endpoint for the S3 SDK to communicate (AWS S3 optional).
	S3EndpointOverride *string `yaml:"s3-endpoint-override,omitempty" json:"s3-endpoint-override,omitempty" example:"http://host.docker.internal:9000"`
	// The log level of the AWS S3 SDK (AWS S3 optional).
	S3LogLevel *string `yaml:"s3-log-level,omitempty" json:"s3-log-level,omitempty" default:"FATAL" enum:"OFF,FATAL,ERROR,WARN,INFO,DEBUG,TRACE"`
	// The minimum size in bytes of individual S3 UploadParts
	MinPartSize int `yaml:"min_part_size,omitempty" json:"min_part_size,omitempty" example:"10" default:"5242880"`
	// The maximum number of simultaneous requests from S3.
	MaxConnsPerHost int `yaml:"max_async_connections,omitempty" json:"max_async_connections,omitempty" example:"16"`
}

// Validate checks if the S3Storage is valid.
func (s *S3Storage) Validate() error {
	if s.Bucket == "" {
		return errors.New("S3 bucket is not specified")
	}
	if s.Path == "" {
		return errors.New("S3 storage path is not specified")
	}
	if s.S3Region == "" {
		return errors.New("S3 region is not specified")
	}
	return nil
}

// GcpStorage represents the configuration for GCP storage.
type GcpStorage struct {
	// Path to file containing Service Account JSON Key.
	KeyFile string `yaml:"key-file" json:"key-file" validate:"required"`
	// For GCP storage bucket is not part of the path as in S3.
	// So we should set it separately.
	BucketName string `yaml:"bucket-name" json:"bucket-name" validate:"required"`
	// The root path for the backup repository.
	Path string `yaml:"path" json:"path" example:"backups" validate:"required"`
	// Alternative url.
	// It is not recommended to use an alternate URL in a production environment.
	Endpoint string `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
}

// Validate checks if the GcpStorage is valid.
func (g *GcpStorage) Validate() error {
	if g.KeyFile == "" {
		return errors.New("GCP key file is not specified")
	}
	if g.BucketName == "" {
		return errors.New("GCP bucket name is not specified")
	}
	if g.Path == "" {
		return errors.New("GCP storage path is not specified")
	}
	return nil
}

// ToModel converts the Storage DTO to its corresponding model.
func (s *Storage) ToModel() model.Storage {
	if s.LocalStorage != nil {
		return &model.LocalStorage{
			Path: s.LocalStorage.Path,
		}
	}
	if s.S3Storage != nil {
		return &model.S3Storage{
			Bucket:             s.S3Storage.Bucket,
			Path:               s.S3Storage.Path,
			S3Region:           s.S3Storage.S3Region,
			S3Profile:          s.S3Storage.S3Profile,
			S3EndpointOverride: s.S3Storage.S3EndpointOverride,
			S3LogLevel:         s.S3Storage.S3LogLevel,
			MinPartSize:        s.S3Storage.MinPartSize,
			MaxConnsPerHost:    s.S3Storage.MaxConnsPerHost,
		}
	}
	if s.GcpStorage != nil {
		return &model.GcpStorage{
			KeyFile:    s.GcpStorage.KeyFile,
			BucketName: s.GcpStorage.BucketName,
			Path:       s.GcpStorage.Path,
			Endpoint:   s.GcpStorage.Endpoint,
		}
	}

	slog.Info("error converting storage dto to model: no storage configuration provided")
	return nil
}

// NewStorageFromModel creates a new Storage DTO from the model.
func NewStorageFromModel(m model.Storage) *Storage {
	switch s := m.(type) {
	case *model.LocalStorage:
		return &Storage{
			LocalStorage: &LocalStorage{
				Path: s.Path,
			},
		}
	case *model.S3Storage:
		return &Storage{
			S3Storage: &S3Storage{
				Bucket:             s.Bucket,
				Path:               s.Path,
				S3Region:           s.S3Region,
				S3Profile:          s.S3Profile,
				S3EndpointOverride: s.S3EndpointOverride,
				S3LogLevel:         s.S3LogLevel,
				MinPartSize:        s.MinPartSize,
				MaxConnsPerHost:    s.MaxConnsPerHost,
			},
		}
	case *model.GcpStorage:
		return &Storage{
			GcpStorage: &GcpStorage{
				KeyFile:    s.KeyFile,
				BucketName: s.BucketName,
				Path:       s.Path,
				Endpoint:   s.Endpoint,
			},
		}
	default:
		return nil
	}
}

// NewStorageFromReader creates a new Storage object from a given reader
func NewStorageFromReader(r io.Reader, format SerializationFormat) (*Storage, error) {
	s := &Storage{}
	if err := Deserialize(s, r, format); err != nil {
		return nil, err
	}

	if err := s.Validate(); err != nil {
		return nil, err
	}

	return s, nil
}
