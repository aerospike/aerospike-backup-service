package configuration

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/service"
)

// StorageManager implement Manager interface.
// it stores service configuration in provided Storage (Local, s3 aws etc.)
type StorageManager struct {
	storage model.Storage
}

// NewStorageManager returns new instance of StorageManager
func NewStorageManager(configStorage model.Storage) Manager {
	return &StorageManager{
		storage: configStorage,
	}
}

func (m *StorageManager) ReadConfiguration(ctx context.Context) (io.ReadCloser, error) {
	content, err := service.ReadFile(ctx, m.storage, "")
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration file: %w", err)
	}

	return io.NopCloser(bytes.NewReader(content)), nil
}

func (m *StorageManager) WriteConfiguration(ctx context.Context, config *model.Config) error {
	configDto := dto.NewConfigFromModel(config)
	data, err := dto.Serialize(configDto, dto.YAML)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration data: %w", err)
	}

	return service.WriteFile(ctx, m.storage, "", data)
}
