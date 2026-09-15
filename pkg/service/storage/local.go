package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/io/storage/local"
	"github.com/aerospike/backup-go/io/storage/options"
)

type LocalStorageAccessor struct{}

func NewLocalStorageAccessor() *LocalStorageAccessor {
	return &LocalStorageAccessor{}
}

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
	ctx context.Context, storage model.Storage, opts ...options.Opt,
) (backup.Writer, error) {
	s := storage.(*model.LocalStorage)
	if s.MinPartSize != nil {
		opts = append(opts, options.WithChunkSize(*s.MinPartSize))
	}

	return local.NewWriter(ctx, opts...)
}

// probe checks that the configured path is usable as a backup destination. The local
// writer creates missing directories itself, so what has to hold is that the nearest
// existing ancestor of the path is a directory this process can write to.
func (a *LocalStorageAccessor) probe(ctx context.Context, storage model.Storage) error {
	path := storage.(*model.LocalStorage).Path

	dir, err := nearestExistingDir(ctx, path)
	if err != nil {
		return err
	}

	probe, err := os.CreateTemp(dir, connectivityProbeKey+"-*")
	if err != nil {
		return fmt.Errorf("local storage write permission check failed for %q: %w", dir, err)
	}
	defer func() { _ = os.Remove(probe.Name()) }()

	if err := probe.Close(); err != nil {
		return fmt.Errorf("local storage write permission check failed for %q: %w", dir, err)
	}

	return nil
}

// nearestExistingDir walks up from path to the first component that exists and reports
// an error unless that component is a directory.
//
// It checks ctx between components rather than around each one: a stat that blocks in the
// kernel - a hung network mount - cannot be interrupted by a context at all, so the most
// this can do is stop walking once the deadline has passed.
func nearestExistingDir(ctx context.Context, path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("local storage path %q cannot be resolved: %w", path, err)
	}

	for dir := abs; ; dir = filepath.Dir(dir) {
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("local storage path %q could not be checked: %w", path, err)
		}

		info, err := os.Stat(dir)
		switch {
		case os.IsNotExist(err):
			if parent := filepath.Dir(dir); parent != dir {
				continue // not created yet; the writer will create it.
			}

			return "", fmt.Errorf("local storage path %q has no existing parent directory", path)
		case err != nil:
			return "", fmt.Errorf("local storage path %q is not accessible: %w", path, err)
		case !info.IsDir():
			return "", fmt.Errorf("local storage path %q is not a directory", dir)
		}

		return dir, nil
	}
}
