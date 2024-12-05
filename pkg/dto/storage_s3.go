package dto

import (
	"errors"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
)

// S3Storage represents the configuration for S3 storage.
//
//nolint:lll
type S3Storage struct {
	// The S3 bucket name.
	Bucket string `yaml:"bucket" json:"bucket" validate:"required"`
	// The root path for the backup repository within the bucket.
	// If not specified, backups will be saved in the bucket's root.
	Path string `yaml:"path,omitempty" json:"path,omitempty" example:"backups"`
	// The S3 region string.
	S3Region string `yaml:"s3-region" json:"s3-region" example:"eu-central-1" validate:"required"`
	// The S3 profile name (AWS S3 optional).
	S3Profile string `yaml:"s3-profile,omitempty" json:"s3-profile,omitempty" example:"default"`
	// An alternative endpoint for the S3 SDK to communicate (AWS S3 optional).
	S3EndpointOverride *string `yaml:"s3-endpoint-override,omitempty" json:"s3-endpoint-override,omitempty" example:"http://host.docker.internal:9000"`
	// The log level of the AWS S3 SDK (AWS S3 optional).
	S3LogLevel *string `yaml:"s3-log-level,omitempty" json:"s3-log-level,omitempty" default:"FATAL" enum:"OFF,FATAL,ERROR,WARN,INFO,DEBUG,TRACE"`
	// The minimum size in bytes of individual S3 UploadParts.
	MinPartSize int `yaml:"min_part_size,omitempty" json:"min_part_size,omitempty" example:"10" default:"5242880"`
	// The maximum number of simultaneous requests from S3.
	MaxConnsPerHost int `yaml:"max_async_connections,omitempty" json:"max_async_connections,omitempty" example:"16"`
	// Secret Agent configuration (optional). Link to one of preconfigured agents.
	SecretAgentName *string `yaml:"secret-agent-name,omitempty" json:"secret-agent-name,omitempty"`
	// Secret Agent configuration (optional).
	SecretAgent *SecretAgent `yaml:"secret-agent,omitempty" json:"secret-agent,omitempty"`
	// Access Key ID for authentication with S3 StaticCredentialsProvider.
	// Can be a path in secret agent or an actual value.
	AccessKeyID *string `yaml:"access-key-id" json:"access-key-id"`
	// Secret Access Key for authentication with S3 StaticCredentialsProvider.
	// Can be a path in secret agent or an actual value.
	SecretAccessKey *string `yaml:"secret-access-key" json:"secret-access-key"`
}

// Validate checks if the S3Storage is valid.
func (s *S3Storage) Validate() error {
	if s.Bucket == "" {
		return errors.New("S3 bucket is not specified")
	}
	if s.S3Region == "" {
		return errors.New("S3 region is not specified")
	}

	if s.AccessKeyID != nil && s.SecretAccessKey == nil {
		return fmt.Errorf("access-key-id is set but secret-access-key is missing")
	}
	if s.AccessKeyID == nil && s.SecretAccessKey != nil {
		return fmt.Errorf("secret-access-key is set but access-key-id is missing")
	}

	if err := validateSecretAgent(s.SecretAgent, s.SecretAgentName); err != nil {
		return err
	}

	return nil
}

func (s *S3Storage) ToModel(config *model.Config) (*model.S3Storage, error) {
	var auth *model.S3Authentication
	if s.AccessKeyID != nil {
		agent, err := config.ResolveSecretAgent(s.SecretAgentName, s.SecretAgent.ToModel())
		if err != nil {
			return nil, err
		}

		auth = &model.S3Authentication{
			KeyIDSecret:     *s.AccessKeyID,
			AccessKeySecret: *s.SecretAccessKey,
			SecretAgent:     agent,
		}
	}
	return &model.S3Storage{
		Path:               s.Path,
		Bucket:             s.Bucket,
		S3Region:           s.S3Region,
		S3Profile:          s.S3Profile,
		S3EndpointOverride: s.S3EndpointOverride,
		S3LogLevel:         s.S3LogLevel,
		MinPartSize:        s.MinPartSize,
		MaxConnsPerHost:    s.MaxConnsPerHost,
		Auth:               auth,
	}, nil
}

func newS3StorageFromModel(s *model.S3Storage, config *model.Config) *S3Storage {
	result := &S3Storage{
		Bucket:             s.Bucket,
		Path:               s.Path,
		S3Region:           s.S3Region,
		S3Profile:          s.S3Profile,
		S3EndpointOverride: s.S3EndpointOverride,
		S3LogLevel:         s.S3LogLevel,
		MinPartSize:        s.MinPartSize,
		MaxConnsPerHost:    s.MaxConnsPerHost,
	}
	if s.Auth != nil {
		result.SecretAgentName, result.SecretAgent = ResolveSecretAgentFromModel(s.Auth.SecretAgent, config)
		result.AccessKeyID = &s.Auth.KeyIDSecret
		result.SecretAccessKey = &s.Auth.AccessKeySecret
	}
	return result
}
