package dto

import (
	"errors"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
)

// GcpStorage represents the configuration for GCP storage.
type GcpStorage struct {
	SecretAgentConfig `yaml:",inline"`
	// Path to the file containing the service account key in JSON format.
	KeyFile string `yaml:"key-file-path,omitempty" json:"key-file-path,omitempty"`
	// Key is the service account key in JSON format.
	// This is sensitive information. Can be a path in secret agent or an actual value.
	Key string `yaml:"key,omitempty" json:"key,omitempty"`
	// GCP storage bucket name.
	BucketName string `yaml:"bucket-name" json:"bucket-name" validate:"required"`
	// The root path for the backup repository. If not specified, backups will be saved in the bucket's root.
	Path string `yaml:"path,omitempty" json:"path,omitempty" example:"backups"`
	// Alternative url.
	// It is not recommended to use an alternate URL in a production environment.
	Endpoint string `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
}

// Validate checks if the GcpStorage is valid.
func (s *GcpStorage) Validate() error {
	if s.BucketName == "" {
		return errors.New("GCP bucket name is not specified")
	}
	if s.KeyFile != "" && s.Key != "" {
		return errors.New("key-file-path and key-json are mutually exclusive")
	}
	return nil
}

func (s *GcpStorage) toModel(config *model.Config) (model.Storage, error) {
	agent, err := config.ResolveSecretAgent(s.SecretAgentName, s.SecretAgent.ToModel())
	if err != nil {
		return nil, err
	}

	return &model.GcpStorage{
		KeyFile:     s.KeyFile,
		BucketName:  s.BucketName,
		Path:        s.Path,
		Endpoint:    s.Endpoint,
		KeyJSON:     s.Key,
		SecretAgent: agent,
	}, nil
}

func newGcpStorageFromModel(s *model.GcpStorage, config *model.Config) *GcpStorage {
	return &GcpStorage{
		KeyFile:           s.KeyFile,
		BucketName:        s.BucketName,
		Path:              s.Path,
		Endpoint:          s.Endpoint,
		Key:               s.KeyJSON,
		SecretAgentConfig: ResolveSecretAgentFromModel(s.SecretAgent, config),
	}
}
