package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go"
	ioStorage "github.com/aerospike/backup-go/io/storage"
	"github.com/aerospike/backup-go/models"
)

// CreateFileReader creates a reader for a file in the specified storage.
func CreateFileReader(
	ctx context.Context,
	storage model.Storage,
	path string,
	opts ...ioStorage.Opt,
) (backup.StreamingReader, error) {
	return getAccessor(storage).createReader(ctx, storage,
		append(opts, ioStorage.WithFile(filepath.Join(storage.GetPath(), path)))...)
}

// CreateDirReader creates a reader for a folder in the specified storage.
func CreateDirReader(
	ctx context.Context,
	storage model.Storage,
	path string,
	opts ...ioStorage.Opt,
) (backup.StreamingReader, error) {
	opts = append(opts,
		ioStorage.WithDir(filepath.Join(storage.GetPath(), path)),
		ioStorage.WithNestedDir())
	return getAccessor(storage).createReader(ctx, storage, opts...)
}

// CreateFileWriter creates a writer for a file in the specified storage.
func CreateFileWriter(
	ctx context.Context,
	storage model.Storage,
	path string,
	opts ...ioStorage.Opt,
) (backup.Writer, error) {
	return getAccessor(storage).createWriter(ctx, storage,
		append(opts, ioStorage.WithFile(filepath.Join(storage.GetPath(), path)))...)
}

// CreateDirWriter creates a writer for a folder in the specified storage.
func CreateDirWriter(
	ctx context.Context,
	storage model.Storage,
	path string,
	opts ...ioStorage.Opt,
) (backup.Writer, error) {
	return getAccessor(storage).createWriter(ctx, storage,
		append(opts, ioStorage.WithDir(filepath.Join(storage.GetPath(), path)))...)
}

func ReadFile(ctx context.Context, storage model.Storage, filepath string) ([]byte, error) {
	reader, err := CreateFileReader(ctx, storage, filepath)

	if err != nil {
		return nil, err
	}

	readersCh := make(chan models.File, 1)
	errorsCh := make(chan error, 1)
	go reader.StreamFiles(ctx, readersCh, errorsCh)

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

func ReadFiles(ctx context.Context, storage model.Storage, path string, filterStr string, fromTime *time.Time,
) ([]*bytes.Buffer, error) {
	var startScanFrom string
	if fromTime != nil {
		fromTimeStr := strconv.FormatInt(fromTime.UnixMilli()-1, 10) // -1 to ensure filter is greater or equal.
		startScanFrom = filepath.Join(path, fromTimeStr)
	}

	reader, err := CreateDirReader(ctx, storage, path,
		ioStorage.WithValidator(newNameValidator(filterStr)),
		ioStorage.WithStartAfter(startScanFrom))
	if err != nil {
		return nil, fmt.Errorf("failed to create reader: %w", err)
	}

	readersCh := make(chan models.File, 1)
	errorsCh := make(chan error, 1)

	go reader.StreamFiles(ctx, readersCh, errorsCh)

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

func WriteFile(ctx context.Context, storage model.Storage, fileName, storageClass string, content []byte) error {
	writer, err := CreateFileWriter(ctx, storage, fileName, ioStorage.WithStorageClass(storageClass))
	if err != nil {
		return fmt.Errorf("failed to create writer: %w", err)
	}

	w, err := writer.NewWriter(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer w.Close()

	_, err = w.Write(content)

	return err
}

func DeleteFolder(ctx context.Context, storage model.Storage, path string) error {
	writer, err := CreateDirWriter(ctx, storage, path, ioStorage.WithNestedDir(), ioStorage.WithRemoveFiles())
	if err != nil {
		return err
	}
	return writer.RemoveFiles(ctx)
}
