package storage

import (
	"context"
	"path/filepath"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/io/local"
)

type LocalStorageAccessor struct{}

func (a *LocalStorageAccessor) supports(storage model.Storage) bool {
	_, ok := storage.(*model.LocalStorage)
	return ok
}

func (a *LocalStorageAccessor) createReader(
	ctx context.Context,
	storage model.Storage,
	path string,
	isFile, sorted, skipDirCheck bool,
	filter Validator,
	_ string,
) (backup.StreamingReader, error) {
	ls := storage.(*model.LocalStorage)
	fullPath := filepath.Join(ls.Path, path)
	opts := []local.Opt{
		local.WithValidator(filter),
		local.WithNestedDir(),
		local.WithSkipDirCheck(),
	}
	if isFile {
		opts = append(opts, local.WithFile(fullPath))
	} else {
		opts = append(opts, local.WithDir(fullPath))
	}
	if skipDirCheck {
		opts = append(opts, local.WithSkipDirCheck())
	}
	if sorted {
		opts = append(opts, local.WithSorting())
	}

	return local.NewReader(ctx, opts...)
}

func (a *LocalStorageAccessor) createWriter(
	ctx context.Context, storage model.Storage, path string, isFile, isRemoveFiles, withNested bool,
) (backup.Writer, error) {
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
