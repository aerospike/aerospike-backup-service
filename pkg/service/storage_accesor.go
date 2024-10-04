package service

import (
	"bytes"
	"context"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/service/storage"
)

func ReadFile(ctx context.Context, s model.Storage, filepath string) ([]byte, error) {
	return storage.ReadFile(ctx, s, filepath)
}

func readFiles(ctx context.Context, s model.Storage, path string, filter string) ([]*bytes.Buffer, error) {
	return storage.ReadFiles(ctx, s, path, filter)
}

func WriteFile(ctx context.Context, s model.Storage, fileName string, content []byte) error {
	return storage.WriteFile(ctx, s, fileName, content)
}

func DeleteFolder(ctx context.Context, s model.Storage, path string) error {
	return storage.DeleteFolder(ctx, s, path)
}
