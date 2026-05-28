package storage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"slices"

	"cloud.google.com/go/storage"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	secrets "github.com/aerospike/aerospike-backup-service/v3/pkg/service/secret"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/aerospike/backup-go"
	gcp "github.com/aerospike/backup-go/io/storage/gcp/storage"
	"github.com/aerospike/backup-go/io/storage/options"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/api/option"
)

const (
	gcpPermissionGet    = "storage.objects.get"
	gcpPermissionList   = "storage.objects.list"
	gcpPermissionCreate = "storage.objects.create"
	gcpPermissionDelete = "storage.objects.delete"
)

type GcpStorageAccessor struct {
	clientMap *collections.LoadingCacheContext[*model.GcpStorage, gcp.Client]
	resolver  secrets.Resolver
}

// clientWrapper wraps *storage.Client to allow proper cleanup with runtime.AddCleanup.
type clientWrapper struct {
	client *storage.Client
}

func (w *clientWrapper) Bucket(name string) *storage.BucketHandle {
	return w.client.Bucket(name)
}

var _ gcp.Client = (*clientWrapper)(nil)

func NewGcpStorageAccessor(ctx context.Context, resolver secrets.Resolver) *GcpStorageAccessor {
	accessor := &GcpStorageAccessor{
		resolver: resolver,
	}
	accessor.clientMap = collections.NewLoadingCacheContext[*model.GcpStorage, gcp.Client](
		ctx,
		accessor.getGcpClient,
		nil,
	)
	return accessor
}

func (a *GcpStorageAccessor) supports(storage model.Storage) bool {
	_, ok := storage.(*model.GcpStorage)
	return ok
}

func (a *GcpStorageAccessor) createReader(
	ctx context.Context,
	storage model.Storage,
	opts ...options.Opt,
) (backup.StreamingReader, error) {
	gcps := storage.(*model.GcpStorage)
	client, err := a.clientMap.Get(ctx, gcps)
	if err != nil {
		return nil, fmt.Errorf("reader failed to create GCP client: %w", err)
	}

	return gcp.NewReader(ctx, client, gcps.BucketName, opts...)
}

func (a *GcpStorageAccessor) createWriter(
	ctx context.Context, storage model.Storage, opts ...options.Opt,
) (backup.Writer, error) {
	gcps := storage.(*model.GcpStorage)
	client, err := a.clientMap.Get(ctx, gcps)
	if err != nil {
		return nil, fmt.Errorf("writer failed to create GCP client: %w", err)
	}

	if gcps.MinPartSize != nil {
		opts = append(opts, options.WithChunkSize(*gcps.MinPartSize))
	}

	return gcp.NewWriter(ctx, client, gcps.BucketName, opts...)
}

func (a *GcpStorageAccessor) getGcpClient(ctx context.Context, g *model.GcpStorage) (gcp.Client, error) {
	opts := make([]option.ClientOption, 0)

	if g.KeyFile != "" {
		opts = append(opts, option.WithAuthCredentialsFile(option.ServiceAccount, g.KeyFile))
	}

	if g.KeyJSON != "" {
		key, err := a.resolver.Resolve(ctx, g.SecretAgent, g.KeyJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to read key json from secret agent: %w", err)
		}

		opts = append(opts, option.WithAuthCredentialsJSON(option.ServiceAccount, []byte(key)))
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
			Max:        model.StorageRetryPolicy.MaxBackoffDuration,
		}),
	)

	if err := checkGcpConnectivity(ctx, client, g.BucketName); err != nil {
		return nil, errors.Join(err, client.Close())
	}

	wrapper := &clientWrapper{client: client}
	// Set cleanup for the client to release allocated resources.
	// We track the wrapper and clean up the underlying client.
	runtime.AddCleanup(wrapper, func(c *storage.Client) {
		_ = c.Close()
	}, client)

	return wrapper, nil
}

func checkGcpConnectivity(ctx context.Context, client gcp.Client, bucket string) error {
	ctx, cancel := context.WithTimeout(ctx, connectivityTimeout)
	defer cancel()

	bkt := client.Bucket(bucket)

	if _, err := bkt.Attrs(ctx); err != nil {
		return fmt.Errorf("gcp storage connectivity check failed: %w", err)
	}

	granted, err := bkt.IAM().TestPermissions(ctx, []string{
		gcpPermissionGet,
		gcpPermissionList,
		gcpPermissionCreate,
		gcpPermissionDelete,
	})

	if err != nil {
		return fmt.Errorf("gcp storage permission check failed: %w", err)
	}

	if !slices.Contains(granted, gcpPermissionList) {
		return fmt.Errorf("gcp storage read permission check failed: missing %s", gcpPermissionList)
	}

	if !slices.Contains(granted, gcpPermissionGet) {
		slog.Warn("gcp storage read permission check failed; restores may fail at runtime",
			slog.String("bucket", bucket),
		)
	}

	if !slices.Contains(granted, gcpPermissionCreate) {
		slog.Warn("gcp storage upload permission check failed; backup writes may fail at runtime",
			slog.String("bucket", bucket),
		)
	}

	if !slices.Contains(granted, gcpPermissionDelete) {
		slog.Warn("gcp storage delete permission check failed; backup writes or cleanup may fail at runtime",
			slog.String("bucket", bucket),
		)
	}

	return nil
}
