package configuration

import (
	"bytes"
	"context"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
)

// storageReaderWriter is the part of storage.Operations used to keep the configuration
// file in a storage backend.
type storageReaderWriter interface {
	// ReadFile reads the content of a file in the specified storage.
	ReadFile(ctx context.Context, storage model.Storage, filepath string) ([]byte, error)
	// WriteDataFile writes a data file to the specified storage.
	WriteDataFile(ctx context.Context, storage model.Storage, fileName string, content []byte) error
}

// storageManager keeps the service configuration in a storage backend (local, S3, GCP, Azure).
type storageManager struct {
	storage     model.Storage
	nsValidator aerospike.NamespaceValidator
	operations  storageReaderWriter
}

var _ Manager = (*storageManager)(nil)

// newStorageManager returns new instance of storageManager.
func newStorageManager(
	configStorage model.Storage,
	nsValidator aerospike.NamespaceValidator,
	operations storageReaderWriter,
) Manager {
	return &storageManager{
		storage:     configStorage,
		nsValidator: nsValidator,
		operations:  operations,
	}
}

func (m *storageManager) Read(ctx context.Context) (*model.Config, error) {
	content, err := m.operations.ReadFile(ctx, m.storage, "")
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration from storage: %w", err)
	}

	return readConfig(ctx, bytes.NewReader(content), m.nsValidator)
}

func (m *storageManager) Write(ctx context.Context, config *model.Config) error {
	var buf bytes.Buffer
	if err := writeConfig(&buf, config); err != nil {
		return err
	}

	if err := m.operations.WriteDataFile(ctx, m.storage, "", buf.Bytes()); err != nil {
		return fmt.Errorf("failed to write configuration to storage %+v: %w", m.storage, err)
	}

	return nil
}
