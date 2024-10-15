package configuration

import (
	"bytes"
	"context"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/service/storage"
)

// storageManager implements Manager interface.
// it stores service configuration in provided Storage (Local, s3 aws etc.)
type storageManager struct {
	storage model.Storage
}

// newStorageManager returns new instance of storageManager
func newStorageManager(configStorage model.Storage) Manager {
	return &storageManager{
		storage: configStorage,
	}
}

func (m *storageManager) Read(ctx context.Context) (*model.Config, error) {
	content, err := storage.ReadFile(ctx, m.storage, "")
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration from storage: %w", err)
	}

	return readConfig(bytes.NewReader(content))
}

func (m *storageManager) Write(ctx context.Context, config *model.Config) error {
	var buf bytes.Buffer
	if err := writeConfig(&buf, config); err != nil {
		return err
	}

	if err := storage.WriteFile(ctx, m.storage, "", buf.Bytes()); err != nil {
		return fmt.Errorf("failed to write configuration to storage %+v: %w", m.storage, err)
	}

	return nil
}
