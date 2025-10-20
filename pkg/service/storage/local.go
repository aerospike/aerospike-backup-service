package storage

import (
	"context"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/io/storage/local"
	"github.com/aerospike/backup-go/io/storage/options"
)

type LocalStorageAccessor struct{}

func (a *LocalStorageAccessor) supports(storage model.Storage) bool {
	_, ok := storage.(*model.LocalStorage)
	return ok
}

func (a *LocalStorageAccessor) createReader(
	ctx context.Context,
	_ model.Storage,
	opts ...options.Opt,
) (backup.StreamingReader, error) {
	return local.NewReader(ctx, opts...)
}

func (a *LocalStorageAccessor) createWriter(
	ctx context.Context, _ model.Storage, opts ...options.Opt,
) (backup.Writer, error) {
	return local.NewWriter(ctx, opts...)
}

func init() {
	registerAccessor(&LocalStorageAccessor{})
}
