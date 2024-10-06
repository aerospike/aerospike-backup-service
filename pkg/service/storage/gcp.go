package storage

import (
	"context"
	"fmt"
	"path/filepath"

	"cloud.google.com/go/storage"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/backup-go"
	gcp "github.com/aerospike/backup-go/io/gcp/storage"
	"google.golang.org/api/option"
)

type GcpStorageAccessor struct{}

func (a *GcpStorageAccessor) supports(storage model.Storage) bool {
	_, ok := storage.(*model.GcpStorage)
	return ok
}

func (a *GcpStorageAccessor) createReader(
	ctx context.Context, storage model.Storage, path string, isFile bool, filter Validator,
) (backup.StreamingReader, error) {
	gcps := storage.(*model.GcpStorage)
	client, err := getGcpClient(ctx, gcps)
	if err != nil {
		return nil, err
	}
	opts := []gcp.Opt{
		gcp.WithValidator(filter),
		gcp.WithNestedDir(),
	}
	fullPath := filepath.Join(gcps.Path, path)
	if isFile {
		opts = append(opts, gcp.WithFile(fullPath))
	} else {
		opts = append(opts, gcp.WithDir(fullPath))
	}
	return gcp.NewReader(ctx, client, gcps.BucketName, opts...)
}

func (a *GcpStorageAccessor) createWriter(
	ctx context.Context, storage model.Storage, path string, isFile, isRemoveFiles, withNested bool,
) (backup.Writer, error) {
	gcps := storage.(*model.GcpStorage)
	client, err := getGcpClient(ctx, gcps)
	if err != nil {
		return nil, err
	}
	fullPath := filepath.Join(gcps.Path, path)
	var opts []gcp.Opt
	if isFile {
		opts = append(opts, gcp.WithFile(fullPath))
	} else {
		opts = append(opts, gcp.WithDir(fullPath))
	}
	if isRemoveFiles {
		opts = append(opts, gcp.WithRemoveFiles())
	}
	if withNested {
		opts = append(opts, gcp.WithNestedDir())
	}
	return gcp.NewWriter(ctx, client, gcps.BucketName, opts...)
}

func init() {
	registerAccessor(&GcpStorageAccessor{})
}

func getGcpClient(ctx context.Context, g *model.GcpStorage) (*storage.Client, error) {
	opts := make([]option.ClientOption, 0)

	if g.KeyFile != "" {
		opts = append(opts, option.WithCredentialsFile(g.KeyFile))
	}

	if g.Endpoint != "" {
		opts = append(opts, option.WithEndpoint(g.Endpoint), option.WithoutAuthentication())
	}

	gcpClient, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCP client: %w", err)
	}

	return gcpClient, nil
}
