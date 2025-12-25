package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/io/storage/options"
	"github.com/aerospike/backup-go/models"
)

var connectivityTimeout = 15 * time.Second

// Manager provides storage operations across different storage backends.
type Manager interface {
	CreateFileReader(ctx context.Context, storage model.Storage, path string, opts ...options.Opt) (backup.StreamingReader, error)
	CreateDirReader(ctx context.Context, storage model.Storage, path string, opts ...options.Opt) (backup.StreamingReader, error)
	CreateFileWriter(ctx context.Context, storage model.Storage, path string, opts ...options.Opt) (backup.Writer, error)
	CreateDirWriter(ctx context.Context, storage model.Storage, path string, opts ...options.Opt) (backup.Writer, error)
	ReadFile(ctx context.Context, storage model.Storage, filepath string) ([]byte, error)
	ReadFiles(ctx context.Context, storage model.Storage, path string, filterStr string) ([]*bytes.Buffer, error)
	ReadFileNames(ctx context.Context, storage model.Storage, path string, filterStr string, fromTime *time.Time) ([]string, error)
	WriteMetadataFile(ctx context.Context, storage model.Storage, fileName string, content []byte) error
	WriteDataFile(ctx context.Context, storage model.Storage, fileName string, content []byte) error
	DeleteFolder(ctx context.Context, storage model.Storage, path string) error
}

// StorageServiceImpl implements Manager using registered accessors.
type StorageServiceImpl struct {
	accessors []Accessor
}

var _ Manager = (*StorageServiceImpl)(nil)

// NewStorageService creates a new storage service with the given accessors.
func NewStorageService(accessors ...Accessor) Manager {
	return &StorageServiceImpl{
		accessors: accessors,
	}
}

// getAccessor returns the appropriate accessor for the given storage.
func (s *StorageServiceImpl) getAccessor(storage model.Storage) Accessor {
	for _, accessor := range s.accessors {
		if accessor.supports(storage) {
			return accessor
		}
	}

	panic(fmt.Sprintf("unsupported storage type %T", storage))
}

// CreateFileReader creates a reader for a file in the specified storage.
func (s *StorageServiceImpl) CreateFileReader(
	ctx context.Context,
	storage model.Storage,
	path string,
	opts ...options.Opt,
) (backup.StreamingReader, error) {
	opts = append(opts,
		options.WithFile(filepath.Join(storage.GetPath(), path)),
		options.WithLogger(slog.Default()))
	return s.getAccessor(storage).createReader(ctx, storage, opts...)
}

// CreateDirReader creates a reader for a folder in the specified storage.
func (s *StorageServiceImpl) CreateDirReader(
	ctx context.Context,
	storage model.Storage,
	path string,
	opts ...options.Opt,
) (backup.StreamingReader, error) {
	opts = append(opts,
		options.WithDir(filepath.Join(storage.GetPath(), path)),
		options.WithNestedDir(),
		options.WithLogger(slog.Default()))
	return s.getAccessor(storage).createReader(ctx, storage, opts...)
}

// CreateFileWriter creates a writer for a file in the specified storage.
func (s *StorageServiceImpl) CreateFileWriter(
	ctx context.Context,
	storage model.Storage,
	path string,
	opts ...options.Opt,
) (backup.Writer, error) {
	opts = append(opts, options.WithFile(filepath.Join(storage.GetPath(), path)))
	return s.getAccessor(storage).createWriter(ctx, storage, opts...)
}

// CreateDirWriter creates a writer for a folder in the specified storage.
func (s *StorageServiceImpl) CreateDirWriter(
	ctx context.Context,
	storage model.Storage,
	path string,
	opts ...options.Opt,
) (backup.Writer, error) {
	opts = append(opts, options.WithDir(filepath.Join(storage.GetPath(), path)))
	return s.getAccessor(storage).createWriter(ctx, storage, opts...)
}

func (s *StorageServiceImpl) ReadFile(ctx context.Context, storage model.Storage, filepath string) ([]byte, error) {
	reader, err := s.CreateFileReader(ctx, storage, filepath)

	if err != nil {
		return nil, err
	}

	readersCh := make(chan models.File, 1)
	errorsCh := make(chan error, 1)
	go reader.StreamFiles(ctx, readersCh, errorsCh, nil)

	select {
	case err := <-errorsCh:
		return nil, err
	case r := <-readersCh:
		defer r.Reader.Close()
		return io.ReadAll(r.Reader)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *StorageServiceImpl) ReadFiles(
	ctx context.Context,
	storage model.Storage,
	path string,
	filterStr string,
) ([]*bytes.Buffer, error) {
	reader, err := s.CreateDirReader(ctx, storage, path,
		options.WithValidator(newNameValidator(filterStr)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create reader: %w", err)
	}

	readersCh := make(chan models.File, 1)
	errorsCh := make(chan error, 1)

	go reader.StreamFiles(ctx, readersCh, errorsCh, nil)

	var files []*bytes.Buffer
	for {
		select {
		case err := <-errorsCh:
			if errors.Is(err, io.EOF) {
				return files, nil
			}
			return nil, err
		case r, ok := <-readersCh:
			if !ok {
				return files, nil
			}
			buf := new(bytes.Buffer)
			_, err := func() (int64, error) {
				defer r.Reader.Close()
				return io.Copy(buf, r.Reader)
			}()
			if err != nil {
				return nil, err
			}
			files = append(files, buf)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (s *StorageServiceImpl) ReadFileNames(
	ctx context.Context, storage model.Storage, path string, filterStr string, fromTime *time.Time,
) ([]string, error) {
	var startScanFrom string
	if fromTime != nil {
		fromTimeStr := fmt.Sprintf("%013d", fromTime.UnixMilli()-1)
		startScanFrom = filepath.Join(storage.GetPath(), path, fromTimeStr)
	}

	reader, err := s.CreateDirReader(ctx, storage, path,
		options.WithValidator(newNameValidator(filterStr)),
		options.WithStartAfter(startScanFrom),
		options.WithNestedDir(),
		options.WithSkipDirCheck(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create reader: %w", err)
	}

	return reader.ListObjects(ctx, filepath.Join(storage.GetPath(), path)+"/")
}

func (s *StorageServiceImpl) WriteMetadataFile(
	ctx context.Context,
	storage model.Storage,
	fileName string,
	content []byte,
) error {
	return s.writeFile(ctx, storage, fileName, storage.GetStorageClass().MetadataClass, content)
}

func (s *StorageServiceImpl) WriteDataFile(
	ctx context.Context,
	storage model.Storage,
	fileName string,
	content []byte,
) error {
	return s.writeFile(ctx, storage, fileName, storage.GetStorageClass().DataClass, content)
}

func (s *StorageServiceImpl) writeFile(
	ctx context.Context,
	storage model.Storage,
	fileName, storageClass string,
	content []byte,
) error {
	writer, err := s.CreateFileWriter(ctx, storage, fileName, options.WithStorageClass(storageClass))
	if err != nil {
		return fmt.Errorf("failed to create writer: %w", err)
	}

	w, err := writer.NewWriter(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	if _, err := w.Write(content); err != nil {
		return errors.Join(fmt.Errorf("failed to write content: %w", err),
			w.Close())
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close file: %w", err)
	}

	return nil
}

func (s *StorageServiceImpl) DeleteFolder(ctx context.Context, storage model.Storage, path string) error {
	writer, err := s.CreateDirWriter(ctx, storage, path, options.WithNestedDir(), options.WithRemoveFiles())
	if err != nil {
		return err
	}
	return writer.RemoveFiles(ctx)
}
