package model

import "errors"

// Storage represents the configuration for a backup storage details.
type Storage struct {
	Type               StorageType `yaml:"type,omitempty" json:"type,omitempty"`
	Path               *string     `yaml:"path,omitempty" json:"path,omitempty"`
	S3Region           *string     `yaml:"s3-region,omitempty" json:"s3-region,omitempty"`
	S3Profile          *string     `yaml:"s3-profile,omitempty" json:"s3-profile,omitempty"`
	S3EndpointOverride *string     `yaml:"s3-endpoint-override,omitempty" json:"s3-endpoint-override,omitempty"`
	S3LogLevel         *string     `yaml:"s3-log-level,omitempty" json:"s3-log-level,omitempty"`
}

// StorageType represents the type of the backup storage.
type StorageType int

const (
	Local StorageType = iota
	S3
)

// Validate validates the storage configuration.
func (s *Storage) Validate() error {
	if s == nil {
		return errors.New("source storage is not specified")
	}
	if s.Path == nil {
		return errors.New("storage path is not specified")
	}
	return nil
}
