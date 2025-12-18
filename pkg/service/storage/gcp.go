package storage

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"

	"cloud.google.com/go/storage"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/aerospike/backup-go"
	gcp "github.com/aerospike/backup-go/io/storage/gcp/storage"
	"github.com/aerospike/backup-go/io/storage/options"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/api/option"
)

// GcpClientHandle wraps the client to manage its lifecycle via AddCleanup.
type GcpClientHandle struct {
	Client *storage.Client
}

type GcpStorageAccessor struct {
	clientMap *collections.LoadingCache[*model.GcpStorage, *GcpClientHandle]
}

func NewGcpStorageAccessor() *GcpStorageAccessor {
	return &GcpStorageAccessor{
		clientMap: collections.NewLoadingCache[*model.GcpStorage, *GcpClientHandle](context.Background(), getGcpClient),
	}
}

func (a *GcpStorageAccessor) supports(storage model.Storage) bool {
	_, ok := storage.(*model.GcpStorage)
	return ok
}

// keepAliveReader ensures the Handle (and thus the cleanup) waits until the Reader is done.
type keepAliveReader struct {
	backup.StreamingReader
	handle *GcpClientHandle
}

func (a *GcpStorageAccessor) createReader(
	ctx context.Context,
	s model.Storage,
	opts ...options.Opt,
) (backup.StreamingReader, error) {
	gcps := s.(*model.GcpStorage)
	handle, err := a.clientMap.GetWithContext(ctx, gcps)
	if err != nil {
		return nil, fmt.Errorf("reader failed to create GCP client: %w", err)
	}

	// Use the inner Client for the actual work
	r, err := gcp.NewReader(ctx, handle.Client, gcps.BucketName, opts...)
	if err != nil {
		return nil, err
	}

	// Return a wrapper that holds the reference to 'handle'
	return &keepAliveReader{StreamingReader: r, handle: handle}, nil
}

// keepAliveWriter ensures the Handle (and thus the cleanup) waits until the Writer is done.
type keepAliveWriter struct {
	backup.Writer
	handle *GcpClientHandle
}

func (a *GcpStorageAccessor) createWriter(
	ctx context.Context, s model.Storage, opts ...options.Opt,
) (backup.Writer, error) {
	gcps := s.(*model.GcpStorage)
	handle, err := a.clientMap.GetWithContext(ctx, gcps)
	if err != nil {
		return nil, fmt.Errorf("writer failed to create GCP client: %w", err)
	}

	if gcps.MinPartSize != nil {
		opts = append(opts, options.WithChunkSize(*gcps.MinPartSize))
	}

	w, err := gcp.NewWriter(ctx, handle.Client, gcps.BucketName, opts...)
	if err != nil {
		return nil, err
	}

	return &keepAliveWriter{Writer: w, handle: handle}, nil
}

func init() {
	registerAccessor(NewGcpStorageAccessor())
}

func getGcpClient(ctx context.Context, g *model.GcpStorage) (*GcpClientHandle, error) {
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
			Max:        model.StorageRetryPolicy.MaxBackoffDuration,
		}),
	)

	handle := &GcpClientHandle{Client: client}

	// REGISTER CLEANUP:
	// 1. We track 'handle'. When 'handle' is unreachable, cleanup runs.
	// 2. We pass 'client' as the argument.
	// 3. IMPORTANT: 'handle' points to 'client', but 'client' does NOT point to 'handle'.
	//    This prevents the reference cycle that would stop GC.
	runtime.AddCleanup(handle, func(c *storage.Client) {
		slog.Debug("Close GCP client", slog.String("storage", g.String()))
		_ = c.Close()
	}, client)

	return handle, nil
}
