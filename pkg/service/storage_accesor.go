package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/io/aws/s3"
	"github.com/aerospike/backup-go/io/local"
	"github.com/aws/aws-sdk-go-v2/config"
	awsS3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

func ReadFile(ctx context.Context, storage model.Storage, filepath string) ([]byte, error) {
	reader, err := readerForStorage(ctx, storage, filepath, true, nil)
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

func readFiles(ctx context.Context, storage model.Storage, path string, filter validator) ([]*bytes.Buffer, error) {
	reader, err := readerForStorage(ctx, storage, path, false, filter)
	if err != nil {
		return nil, err
	}
	readersCh := make(chan io.ReadCloser)
	errorsCh := make(chan error, 1)

	go reader.StreamFiles(ctx, readersCh, errorsCh)

	var files []*bytes.Buffer
	for {
		select {
		case err := <-errorsCh:
			if err == io.EOF {
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
	writer, err := writerForStorage(ctx, fileName, storage, true, false, false)
	if err != nil {
		return err
	}

	w, err := writer.NewWriter(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to create writer: %w", err)
	}
	defer w.Close()

	// Write the content to the file
	_, err = w.Write(content)
	if err != nil {
		return fmt.Errorf("failed to write content: %w", err)
	}

	return nil
}

func DeleteFolder(ctx context.Context, storage model.Storage, path string) error {
	writer, err := writerForStorage(ctx, path, storage, false, true, true)
	if err != nil {
		return err
	}

	return writer.RemoveFiles(ctx)
}

type validator interface {
	Run(fileName string) error
}

type nameValidator struct {
	filter string
}

func (n *nameValidator) Run(path string) error {
	if len(n.filter) == 0 {
		return nil
	}

	if strings.HasSuffix(path, n.filter) {
		return nil
	}

	return fmt.Errorf("skipped by filter '%s'", n.filter)
}

var metadataFilter = &nameValidator{metadataFile}
var configurationFilter = &nameValidator{".conf"}

// readerForStorage instantiates and returns a reader for the restore operation
// according to the specified storage type.
func readerForStorage(ctx context.Context, storage model.Storage, path string, isFile bool, filter validator,
) (backup.StreamingReader, error) {
	switch storage := storage.(type) {
	case *model.LocalStorage:
		fullPath := filepath.Join(storage.Path, path)
		opts := []local.Opt{
			local.WithNestedDir(),
		}
		if filter != nil {
			opts = append(opts, local.WithValidator(filter))
		}
		if isFile {
			opts = append(opts, local.WithFile(fullPath))
		} else {
			opts = append(opts, local.WithDir(fullPath))
		}
		return local.NewReader(opts...)
	case *model.S3Storage:
		client, err := getS3Client(
			ctx, storage.S3Profile, storage.S3Region, storage.S3EndpointOverride, storage.MaxConnsPerHost)
		if err != nil {
			return nil, err
		}
		opts := []s3.Opt{
			s3.WithValidator(filter),
			s3.WithNestedDir(),
		}
		fullPath := filepath.Join(storage.Path, path)
		if isFile {
			opts = append(opts, s3.WithFile(fullPath))
		} else {
			opts = append(opts, s3.WithDir(fullPath))
		}
		return s3.NewReader(ctx, client, storage.Bucket, opts...)
	}
	return nil, fmt.Errorf("unknown storage type %T", storage)
}

// writerForStorage instantiates and returns a writer for the backup operation
// according to the specified storage type.
func writerForStorage(ctx context.Context, path string, storage model.Storage,
	isFile, isRemoveFiles, withNested bool) (backup.Writer, error) {
	switch storage := storage.(type) {
	case *model.LocalStorage:
		fullPath := filepath.Join(storage.Path, path)
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
	case *model.S3Storage:
		client, err := getS3Client(
			ctx, storage.S3Profile, storage.S3Region, storage.S3EndpointOverride, storage.MaxConnsPerHost)
		if err != nil {
			return nil, err
		}
		fullPath := filepath.Join(storage.Path, path)
		var opts []s3.Opt
		if isFile {
			opts = append(opts, s3.WithFile(fullPath))
		} else {
			opts = append(opts, s3.WithDir(fullPath))
		}
		if isRemoveFiles {
			opts = append(opts, s3.WithRemoveFiles())
		}
		if withNested {
			opts = append(opts, s3.WithNestedDir())
		}
		return s3.NewWriter(ctx, client, storage.Bucket, opts...)
	}
	return nil, fmt.Errorf("unknown storage type %T", storage)
}

func getS3Client(ctx context.Context, profile, region string, endpoint *string,
	maxConnsPerHost int) (*awsS3.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedConfigProfile(profile),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, err
	}

	client := awsS3.NewFromConfig(cfg, func(o *awsS3.Options) {
		if endpoint != nil {
			o.BaseEndpoint = endpoint
		}

		o.UsePathStyle = true

		if maxConnsPerHost > 0 {
			o.HTTPClient = &http.Client{
				Transport: &http.Transport{
					MaxConnsPerHost: maxConnsPerHost,
				},
			}
		}
	})

	return client, nil
}
