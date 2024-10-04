package storage

import (
	"context"
	"path/filepath"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/io/local"
)

type LocalStorageAccessor struct{}

func (a *LocalStorageAccessor) supports(storage model.Storage) bool {
	_, ok := storage.(*model.LocalStorage)
	return ok
}

func (a *LocalStorageAccessor) createReader(ctx context.Context, storage model.Storage, path string, isFile bool, filter Validator) (backup.StreamingReader, error) {
	ls := storage.(*model.LocalStorage)
	fullPath := filepath.Join(ls.Path, path)
	opts := []local.Opt{local.WithNestedDir()}
	if filter != nil {
		opts = append(opts, local.WithValidator(filter))
	}
	if isFile {
		opts = append(opts, local.WithFile(fullPath))
	} else {
		opts = append(opts, local.WithDir(fullPath))
	}
	return local.NewReader(opts...)
}

func (a *LocalStorageAccessor) createWriter(ctx context.Context, storage model.Storage, path string, isFile, isRemoveFiles, withNested bool) (backup.Writer, error) {
	ls := storage.(*model.LocalStorage)
	fullPath := filepath.Join(ls.Path, path)
	var opts []local.Opt
	if isFile {
		opts = append(opts, local.WithFile(fullPath))
	} else {
		opts = append(opts, local.WithDir(fullPath))
	}
	if isRemoveFiles {
		opts = append(opts, local.WithRemoveFiles())
	}
	if withNested {
		opts = append(opts, local.WithNestedDir())
	}
	return local.NewWriter(ctx, opts...)
}

func init() {
	registerAccessor(&LocalStorageAccessor{})
}
