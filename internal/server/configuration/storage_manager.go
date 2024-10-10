package configuration

import (
	"bytes"
	"context"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/service/storage"
)

// StorageManager implements Manager interface.
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

func (m *StorageManager) Read(ctx context.Context) (*model.Config, error) {
	content, err := storage.ReadFile(ctx, m.storage, "")
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration from storage: %w", err)
	}

	return readConfig(bytes.NewReader(content))
}

func (m *StorageManager) Write(ctx context.Context, config *model.Config) error {
	data, err := serializeConfig(config)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration data: %w", err)
	}

	if err := storage.WriteFile(ctx, m.storage, "", data); err != nil {
		return fmt.Errorf("failed to write configuration to storage %+v: %w", m.storage, err)
	}

	return nil
}

func (m *StorageManager) Update(ctx context.Context, updateFunc func(*model.Config) error) error {
	config, err := m.Read(ctx)
	if err != nil {
		return fmt.Errorf("failed to read configuration: %w", err)
	}

	if err := updateFunc(config); err != nil {
		return fmt.Errorf("failed to update configuration: %w", err)
	}

	return m.Write(ctx, config)

}
