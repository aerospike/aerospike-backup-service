package storage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
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
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

const (
	gcpPermissionGet    = "storage.objects.get"
	gcpPermissionList   = "storage.objects.list"
	gcpPermissionCreate = "storage.objects.create"
	gcpPermissionDelete = "storage.objects.delete"
)

type GcpStorageAccessor struct {
	clientMap collections.Cache[*model.GcpStorage, gcp.Client]
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

func NewGcpStorageAccessor(resolver secrets.Resolver) *GcpStorageAccessor {
	accessor := &GcpStorageAccessor{
		resolver: resolver,
	}
	accessor.clientMap = collections.NewLoadingCache[*model.GcpStorage, gcp.Client](
		accessor.getGcpClient,
		clientCacheTTL,
	)
	return accessor
}

func (a *GcpStorageAccessor) supports(st model.Storage) bool {
	_, ok := st.(*model.GcpStorage)
	return ok
}

func (a *GcpStorageAccessor) createReader(
	ctx context.Context,
	st model.Storage,
	opts ...options.Opt,
) (backup.StreamingReader, error) {
	gcps := st.(*model.GcpStorage)
	client, err := a.clientMap.Get(ctx, gcps)
	if err != nil {
		return nil, fmt.Errorf("reader failed to create GCP client: %w", err)
	}

	return gcp.NewReader(ctx, client, gcps.BucketName, opts...)
}

func (a *GcpStorageAccessor) createWriter(
	ctx context.Context, st model.Storage, opts ...options.Opt,
) (backup.Writer, error) {
	gcps := st.(*model.GcpStorage)
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
	hasExplicitAuth := false

	if g.KeyFile != "" {
		opts = append(opts, option.WithAuthCredentialsFile(option.ServiceAccount, g.KeyFile))
		hasExplicitAuth = true
	}

	if g.KeyJSON != "" {
		key, err := a.resolver.Resolve(ctx, g.SecretAgent, g.KeyJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to read key json from secret agent: %w", err)
		}

		opts = append(opts, option.WithAuthCredentialsJSON(option.ServiceAccount, []byte(key)))
		hasExplicitAuth = true
	}

	if g.Endpoint != "" {
		opts = append(opts, option.WithEndpoint(g.Endpoint))
		// Without a key-file/key-json, Endpoint names an emulator or other unauthenticated
		// alternative to GCS, so leave real credential resolution off. With one, the caller
		// wants that credential to actually authenticate against the given endpoint (e.g. a
		// storage emulator that does check auth) - forcing WithoutAuthentication here would
		// silently ignore a configured key.
		if !hasExplicitAuth {
			opts = append(opts, option.WithoutAuthentication())
		}
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
		if isNotImplemented(err) {
			// Some GCS-compatible servers (e.g. fake-gcs-server, used in local/CI testing)
			// don't implement testIamPermissions at all. A real GCS bucket never 404s here -
			// it either grants a permission subset or requires storage.buckets.get to reach
			// this point at all - so this can only mean "this server doesn't support the
			// check", not "access is denied".
			slog.Warn("gcp storage permission check unavailable; server does not implement testIamPermissions, "+
				"so read/write access could not be confirmed in advance",
				slog.String("bucket", bucket),
			)
			return nil
		}

		return fmt.Errorf("gcp storage permission check failed: %w", err)
	}

	if !slices.Contains(granted, gcpPermissionList) {
		return fmt.Errorf("gcp storage read permission check failed: missing %s", gcpPermissionList)
	}

	if !slices.Contains(granted, gcpPermissionGet) {
		return fmt.Errorf("gcp storage read permission check failed: missing %s", gcpPermissionGet)
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

// isNotImplemented reports whether err is an HTTP 404 from the GCS API itself (as opposed
// to, say, a network error): the server understood the request enough to route it, but has
// no handler for it.
func isNotImplemented(err error) bool {
	var apiErr *googleapi.Error
	return errors.As(err, &apiErr) && apiErr.Code == http.StatusNotFound
}
