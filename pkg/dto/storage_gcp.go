package dto

import (
	"errors"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
)

// GcpStorage represents the configuration for GCP storage.
type GcpStorage struct {
	SecretAgentConfig
	// Path to file containing Service Account JSON Key.
	KeyFile string `yaml:"key-file" json:"key-file"`
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
		SecretAgent: agent,
	}, nil
}

func newGcpStorageFromModel(s *model.GcpStorage, config *model.Config) *GcpStorage {
	return &GcpStorage{
		KeyFile:           s.KeyFile,
		BucketName:        s.BucketName,
		Path:              s.Path,
		Endpoint:          s.Endpoint,
		SecretAgentConfig: ResolveSecretAgentFromModel(s.SecretAgent, config),
	}
}
