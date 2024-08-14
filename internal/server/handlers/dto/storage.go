package dto

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/aerospike/aerospike-backup-service/pkg/model"
	"github.com/aws/smithy-go/ptr"
)

// Storage represents the configuration for a backup storage details.
// @Description Storage represents the configuration for a backup storage details.
type Storage struct {
	// The type of the storage provider
	Type StorageType `json:"type" enums:"local,aws-s3"`
	// The root path for the backup repository.
	Path *string `json:"path,omitempty" example:"backups"`
	// The S3 region string (AWS S3 optional).
	S3Region *string `json:"s3-region,omitempty" example:"eu-central-1"`
	// The S3 profile name (AWS S3 optional).
	S3Profile *string `json:"s3-profile,omitempty" example:"default"`
	// An alternative endpoint for the S3 SDK to communicate (AWS S3 optional).
	S3EndpointOverride *string `json:"s3-endpoint-override,omitempty" example:"http://host.docker.internal:9000"`
	// The log level of the AWS S3 SDK (AWS S3 optional).
	S3LogLevel *string `json:"s3-log-level,omitempty" default:"FATAL" enum:"OFF,FATAL,ERROR,WARN,INFO,DEBUG,TRACE"`
	// The minimum size in bytes of individual S3 UploadParts
	MinPartSize int `json:"min_part_size,omitempty" example:"10" default:"5242880"`
	// The maximum number of simultaneous requests from S3.
	MaxConnsPerHost int `json:"max_async_connections,omitempty" example:"16"`
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

// MapStorageFromDTO maps Storage to model.Storage
func MapStorageFromDTO(dto Storage) model.Storage {
	return model.Storage{
		Type:               model.StorageType(dto.Type),
		Path:               dto.Path,
		S3Region:           dto.S3Region,
		S3Profile:          dto.S3Profile,
		S3EndpointOverride: dto.S3EndpointOverride,
		S3LogLevel:         dto.S3LogLevel,
		MinPartSize:        dto.MinPartSize,
		MaxConnsPerHost:    dto.MaxConnsPerHost,
	}
}
