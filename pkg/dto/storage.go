package dto

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"slices"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/util"
	"github.com/aws/smithy-go/ptr"
)

// Storage represents the configuration for a backup storage details.
// @Description Storage represents the configuration for a backup storage details.
//
//nolint:lll
type Storage struct {
	// The type of the storage provider
	Type StorageType `yaml:"type" json:"type" enums:"local,aws-s3" validate:"required"`
	// The root path for the backup repository.
	Path *string `yaml:"path,omitempty" json:"path,omitempty" example:"backups" validate:"required"`
	// The S3 region string (AWS S3 optional).
	S3Region *string `yaml:"s3-region,omitempty" json:"s3-region,omitempty" example:"eu-central-1"`
	// The S3 profile name (AWS S3 optional).
	S3Profile *string `yaml:"s3-profile,omitempty" json:"s3-profile,omitempty" example:"default"`
	// An alternative endpoint for the S3 SDK to communicate (AWS S3 optional).
	S3EndpointOverride *string `yaml:"s3-endpoint-override,omitempty" json:"s3-endpoint-override,omitempty" example:"http://host.docker.internal:9000"`
	// The log level of the AWS S3 SDK (AWS S3 optional).
	S3LogLevel *string `yaml:"s3-log-level,omitempty" json:"s3-log-level,omitempty" default:"FATAL" enum:"OFF,FATAL,ERROR,WARN,INFO,DEBUG,TRACE"`
	// The minimum size in bytes of individual S3 UploadParts
	MinPartSize int `yaml:"min_part_size,omitempty" json:"min_part_size,omitempty" example:"10" default:"5242880"`
	// The maximum number of simultaneous requests from S3.
	MaxConnsPerHost int `yaml:"max_async_connections,omitempty" json:"max_async_connections,omitempty" example:"16"`
}

// StorageType represents the type of the backup storage.
// @Description StorageType represents the type of the backup storage.
type StorageType string

const (
	Local              StorageType = "local"
	S3                 StorageType = "aws-s3"
	MinAllowedPartSize             = 5 * 1024 * 1024 // 5 MB in bytes
)

var validS3LogLevels = []string{"OFF", "FATAL", "ERROR", "WARN", "INFO", "DEBUG", "TRACE"}

// Validate validates the storage configuration.
func (s *Storage) Validate() error {
	if s == nil {
		return errors.New("source storage is not specified")
	}
	if s.Path == nil || len(*s.Path) == 0 {
		return errors.New("storage path is not specified")
	}
	if err := s.validateType(); err != nil {
		return err
	}
	if s.Type == S3 {
		if s.S3Region == nil || len(*s.S3Region) == 0 {
			return errors.New("s3 region is not specified")
		}

		_, err := url.Parse(*s.Path)
		if err != nil {
			return fmt.Errorf("failed to parse S3 storage path: %w", err)
		}
	}
	if s.S3LogLevel != nil &&
		!slices.Contains(validS3LogLevels, strings.ToUpper(*s.S3LogLevel)) {
		return errors.New("invalid s3 log level")
	}
	if s.MinPartSize != 0 && s.MinPartSize < MinAllowedPartSize {
		return fmt.Errorf("min_part_size must be at least %d bytes", MinAllowedPartSize)
	}
	if s.MaxConnsPerHost < 0 {
		return errors.New("max_async_connections must not be negative")
	}
	return nil
}

// validateType validates the storage provider type.
func (s *Storage) validateType() error {
	s.Type = StorageType(strings.ToLower(string(s.Type)))
	switch s.Type {
	case Local, S3:
		return nil
	default:
		return fmt.Errorf("invalid storage type: %v", s.Type)
	}
}

// SetDefaultProfile sets the "default" profile if not set.
func (s *Storage) SetDefaultProfile() {
	if s.Type == S3 && s.S3Profile == nil {
		s.S3Profile = ptr.String("default")
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

func (s *Storage) fromModel(m model.Storage) {
	switch m := m.(type) {
	case *model.LocalStorage:
		s.Type = Local
		s.Path = m.Path
	case *model.S3Storage:
		s.Type = S3
		s.Path = &m.Path
		s.S3Region = &m.S3Region
		s.S3Profile = &m.S3Profile
		s.S3EndpointOverride = m.S3EndpointOverride
		s.S3LogLevel = m.S3LogLevel
		s.MinPartSize = m.MinPartSize
		s.MaxConnsPerHost = m.MaxConnsPerHost
	}
}

func NewStorageFromModel(m model.Storage) *Storage {
	if m == nil {
		return nil
	}

	var s Storage
	s.fromModel(m)
	return &s
}

func (s *Storage) ToModel() model.Storage {
	switch s.Type {
	case Local:
		return &model.LocalStorage{
			Path: s.Path,
		}
	case S3:
		// storage path is already validated.
		bucket, parsedPath, _ := util.ParseS3Path(*s.Path)
		profile := "default"
		if s.S3Profile != nil {
			profile = *s.S3Profile
		}

		return &model.S3Storage{
			Path:               parsedPath,
			Bucket:             bucket,
			S3Region:           *s.S3Region,
			S3Profile:          profile,
			S3EndpointOverride: s.S3EndpointOverride,
			S3LogLevel:         s.S3LogLevel,
			MinPartSize:        s.MinPartSize,
			MaxConnsPerHost:    s.MaxConnsPerHost,
		}
	default:
		return nil
	}
}
