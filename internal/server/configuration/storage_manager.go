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

type StorageManager struct {
	storage model.Storage
}

func NewStorageManager(configStorage model.Storage) *StorageManager {
	return &StorageManager{
		storage: configStorage,
	}
}

func (m *StorageManager) ReadConfiguration() (io.ReadCloser, error) {
	content, err := service.ReadOneFile(context.TODO(), m.storage, "")
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration file: %w", err)
	}

	return io.NopCloser(bytes.NewReader(content)), nil
}

func (m *StorageManager) WriteConfiguration(config *model.Config) error {
	writer, err := service.WriterForStorage(context.TODO(), "", m.storage, true, false, false)
	if err != nil {
		return err
	}
	newWriter, err := writer.NewWriter(context.TODO(), "")
	if err != nil {
		return err
	}
	defer newWriter.Close()

	configDto := dto.NewConfigFromModel(config)
	data, err := dto.Serialize(configDto, dto.YAML)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration data: %w", err)
	}

	_, err = newWriter.Write(data)
	return err
}
