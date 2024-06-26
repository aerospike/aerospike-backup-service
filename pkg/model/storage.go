package model

import (
	"errors"
	"fmt"
	"slices"
	"strings"

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
}

// StorageType represents the type of the backup storage.
// @Description StorageType represents the type of the backup storage.
type StorageType string

const (
	Local StorageType = "local"
	S3    StorageType = "aws-s3"
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
