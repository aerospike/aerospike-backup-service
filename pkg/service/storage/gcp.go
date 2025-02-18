package storage

import (
	"context"
	"fmt"

	"cloud.google.com/go/storage"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go"
	ioStorage "github.com/aerospike/backup-go/io/storage"
	gcp "github.com/aerospike/backup-go/io/storage/gcp/storage"
	"google.golang.org/api/option"
)

type GcpStorageAccessor struct{}

func (a *GcpStorageAccessor) supports(storage model.Storage) bool {
	_, ok := storage.(*model.GcpStorage)
	return ok
}

func (a *GcpStorageAccessor) createReader(
	ctx context.Context,
	storage model.Storage,
	opts ...ioStorage.Opt,
) (backup.StreamingReader, error) {
	gcps := storage.(*model.GcpStorage)
	client, err := getGcpClient(ctx, gcps)
	if err != nil {
		return nil, fmt.Errorf("reader failed to create GCP client: %w", err)
	}

	return gcp.NewReader(ctx, client, gcps.BucketName, opts...)
}

func (a *GcpStorageAccessor) createWriter(
	ctx context.Context, storage model.Storage, opts ...ioStorage.Opt,
) (backup.Writer, error) {
	gcps := storage.(*model.GcpStorage)
	client, err := getGcpClient(ctx, gcps)
	if err != nil {
		return nil, fmt.Errorf("writer failed to create GCP client: %w", err)
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
	if g.KeyJSON != "" {
		key, err := g.SecretAgent.Read(g.KeyJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to read key json from secret agent: %w", err)
		}

		opts = append(opts, option.WithCredentialsJSON([]byte(key)))
	}

	if g.Endpoint != "" {
		opts = append(opts, option.WithEndpoint(g.Endpoint), option.WithoutAuthentication())
	}

	return storage.NewClient(ctx, opts...)
}
