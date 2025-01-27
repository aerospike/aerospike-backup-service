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
)

// CreateReader creates a reader for a path in the specified storage.
func CreateReader(
	ctx context.Context, storage model.Storage, path string, isFile bool, v Validator, startScanFrom string,
) (backup.StreamingReader, error) {
	return getAccessor(storage).createReader(ctx, storage, path, isFile, v, startScanFrom)
}

// CreateWriter creates a writer for a path in the specified storage.
func CreateWriter(ctx context.Context, storage model.Storage, path string, isFile, isRemoveFiles, withNested bool,
) (backup.Writer, error) {
	return getAccessor(storage).createWriter(ctx, storage, path, isFile, isRemoveFiles, withNested)
}

func ReadFile(ctx context.Context, storage model.Storage, filepath string) ([]byte, error) {
	reader, err := CreateReader(ctx, storage, filepath, true, nil, "")
	if err != nil {
		return nil, err
	}

	readersCh := make(chan io.ReadCloser, 1)
	errorsCh := make(chan error, 1)
	go reader.StreamFiles(ctx, readersCh, errorsCh)

	select {
	case err := <-errorsCh:
		return nil, err
	case r := <-readersCh:
		defer r.Close()
		return io.ReadAll(r)
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

	reader, err := CreateReader(ctx, storage, path, false, newNameValidator(filterStr), startScanFrom)
	if err != nil {
		return nil, fmt.Errorf("failed to create reader: %w", err)
	}

	readersCh := make(chan io.ReadCloser, 1)
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
				defer r.Close()
				return io.Copy(buf, r)
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

func WriteFile(ctx context.Context, storage model.Storage, fileName string, content []byte) error {
	writer, err := CreateWriter(ctx, storage, fileName, true, false, false)
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
	writer, err := CreateWriter(ctx, storage, path, false, true, true)
	if err != nil {
		return err
	}
	return writer.RemoveFiles(ctx)
}
