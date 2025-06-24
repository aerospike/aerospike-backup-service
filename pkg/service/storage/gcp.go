package storage

import (
	"context"
	"fmt"
	"runtime"

	"cloud.google.com/go/storage"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	"github.com/aerospike/backup-go"
	ioStorage "github.com/aerospike/backup-go/io/storage"
	gcp "github.com/aerospike/backup-go/io/storage/gcp/storage"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/api/option"
)

type GcpStorageAccessor struct {
	clientMap *util.LoadingCache[*model.GcpStorage, *storage.Client]
}

func NewGcpStorageAccessor() *GcpStorageAccessor {
	return &GcpStorageAccessor{
		clientMap: util.NewLoadingCache[*model.GcpStorage, *storage.Client](context.Background(), getGcpClient),
	}
}

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
	client, err := a.clientMap.GetWithContext(ctx, gcps)
	if err != nil {
		return nil, fmt.Errorf("reader failed to create GCP client: %w", err)
	}

	return gcp.NewReader(ctx, client, gcps.BucketName, opts...)
}

func (a *GcpStorageAccessor) createWriter(
	ctx context.Context, storage model.Storage, opts ...ioStorage.Opt,
) (backup.Writer, error) {
	gcps := storage.(*model.GcpStorage)
	client, err := a.clientMap.GetWithContext(ctx, gcps)
	if err != nil {
		return nil, fmt.Errorf("writer failed to create GCP client: %w", err)
	}

	if gcps.MinPartSize != nil {
		opts = append(opts, ioStorage.WithChunkSize(*gcps.MinPartSize))
	}

	return gcp.NewWriter(ctx, client, gcps.BucketName, opts...)
}

func init() {
	registerAccessor(NewGcpStorageAccessor())
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

	client, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCP client: %w", err)
	}

	client.SetRetry(
		storage.WithPolicy(storage.RetryAlways),
		storage.WithMaxAttempts(int(model.StorageRetryPolicy.MaxRetries)),
		storage.WithBackoff(gax.Backoff{
			Initial:    model.StorageRetryPolicy.BaseTimeout,
			Multiplier: model.StorageRetryPolicy.Multiplier,
			Max:        model.StorageRetryPolicy.MaxDuration,
		}),
	)

	// Set finalizer for the client to release allocated resources.
	// TODO: replace with runtime.AddCleanup when upgrading to go1.24
	runtime.SetFinalizer(client, (*storage.Client).Close)

	return client, nil
}
