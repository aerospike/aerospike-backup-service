package backupexecutor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/backup-go/models"
	"github.com/aerospike/backup-go/pkg/asinfo"
	"golang.org/x/sync/semaphore"
)

const (
	serverBackupStorageTypeS3 = "aws-s3"
	// serverBackupProgressScale matches backup-go GetBackupStatus normalization.
	serverBackupProgressScale = 4096
	serverBackupPollInterval  = time.Second
)

// ServerBackupConfig holds parameters for Aerospike server-side backup.
type ServerBackupConfig struct {
	Namespace   string
	StorageType string
	Bucket      string
	Region      string
	Profile     string
	AccessKey   string
	SecretKey   string
}

// ServerBackupCredentials holds resolved object-storage credentials for server-side backup.
type ServerBackupCredentials struct {
	AccessKey string
	SecretKey string
}

// ServerBackupExecutor starts server-side backups via Aerospike info commands.
type ServerBackupExecutor struct {
	clientManager aerospike.ClientManager
	resolver      credentialsResolver
}

// NewServerBackupExecutor creates an executor for Aerospike server-side backup.
func NewServerBackupExecutor(
	clientManager aerospike.ClientManager,
	resolver credentialsResolver,
) *ServerBackupExecutor {
	return &ServerBackupExecutor{
		clientManager: clientManager,
		resolver:      resolver,
	}
}

// Run starts a server-side backup for the given namespace.
func (e *ServerBackupExecutor) Run(
	ctx context.Context,
	routine *model.BackupRoutine,
	_ model.TimeBounds,
	namespace string,
	_ string,
	scanLimiter *semaphore.Weighted,
	logger *slog.Logger,
) (BackupHandler, error) {
	client, err := e.clientManager.GetClient(ctx, routine.SourceCluster, scanLimiter, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to get backup client: %w", err)
	}

	credentials, err := resolveServerBackupCredentials(ctx, e.resolver, routine.Storage)
	if err != nil {
		e.clientManager.Close(client)
		return nil, fmt.Errorf("failed to resolve server backup credentials: %w", err)
	}

	handler, err := runServerBackup(ctx, client.InfoClient(), namespace, routine, credentials)
	if err != nil {
		e.clientManager.Close(client)
		return nil, err
	}

	return newCloseOnWaitBackupHandler(handler, client, e.clientManager), nil
}

// ServerBackupHandler monitors a server-side backup job started via Aerospike info commands.
type ServerBackupHandler struct {
	infoClient serverBackupInfoClient
	jobID      string
	stats      *models.BackupStats
	progress   atomic.Uint64
	waitErr    error
}

var _ BackupHandler = (*ServerBackupHandler)(nil)

type serverBackupInfoClient interface {
	StartServerBackup(
		ctx context.Context,
		namespace, storage, bucket, region, profile, accessKey, secretKey, endpoint string,
	) (string, error)
	GetBackupStatus(ctx context.Context) (float64, error)
}

// runServerBackup starts a server-side backup and returns a handler for monitoring progress.
func runServerBackup(
	ctx context.Context,
	infoClient serverBackupInfoClient,
	namespace string,
	routine *model.BackupRoutine,
	credentials ServerBackupCredentials,
) (BackupHandler, error) {
	config, err := makeServerBackupConfig(namespace, routine, credentials)
	if err != nil {
		return nil, fmt.Errorf("failed to make server backup config: %w", err)
	}

	jobID, err := infoClient.StartServerBackup(
		ctx,
		config.Namespace,
		config.StorageType,
		config.Bucket,
		config.Region,
		config.Profile,
		config.AccessKey,
		config.SecretKey,
		"host.docker.internal",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start server backup: %w", err)
	}

	stats := models.NewBackupStats()
	stats.Start()
	stats.TotalRecords.Store(serverBackupProgressScale)

	return &ServerBackupHandler{
		infoClient: infoClient,
		jobID:      jobID,
		stats:      stats,
	}, nil
}

func makeServerBackupConfig(
	namespace string,
	routine *model.BackupRoutine,
	credentials ServerBackupCredentials,
) (*ServerBackupConfig, error) {
	s3Storage, ok := routine.Storage.(*model.S3Storage)
	if !ok {
		return nil, fmt.Errorf("server backup requires S3 storage, got %T", routine.Storage)
	}

	config := &ServerBackupConfig{
		Namespace:   namespace,
		StorageType: serverBackupStorageTypeS3,
		Bucket:      s3Storage.Bucket,
		Region:      s3Storage.S3Region,
		Profile:     s3Storage.S3Profile,
		AccessKey:   credentials.AccessKey,
		SecretKey:   credentials.SecretKey,
	}

	return config, nil
}

func resolveServerBackupCredentials(
	ctx context.Context,
	resolver credentialsResolver,
	storage model.Storage,
) (ServerBackupCredentials, error) {
	s3Storage, ok := storage.(*model.S3Storage)
	if !ok {
		return ServerBackupCredentials{}, fmt.Errorf("server backup requires S3 storage, got %T", storage)
	}

	if s3Storage.Auth == nil {
		return ServerBackupCredentials{}, nil
	}

	accessKey, err := resolver.Resolve(ctx, s3Storage.Auth.SecretAgent, s3Storage.Auth.KeyIDSecret)
	if err != nil {
		return ServerBackupCredentials{}, fmt.Errorf("failed to resolve access key ID: %w", err)
	}

	secretKey, err := resolver.Resolve(ctx, s3Storage.Auth.SecretAgent, s3Storage.Auth.AccessKeySecret)
	if err != nil {
		return ServerBackupCredentials{}, fmt.Errorf("failed to resolve secret access key: %w", err)
	}

	return ServerBackupCredentials{
		AccessKey: accessKey,
		SecretKey: secretKey,
	}, nil
}

// Wait polls backup status until the job completes or the context is cancelled.
func (h *ServerBackupHandler) Wait(ctx context.Context) error {
	if h == nil {
		return nil
	}

	ticker := time.NewTicker(serverBackupPollInterval)
	defer ticker.Stop()

	for {
		status, err := h.infoClient.GetBackupStatus(ctx)
		if err != nil && !errors.Is(err, asinfo.ErrNotFound) {
			h.waitErr = fmt.Errorf("failed to get server backup status: %w", err)
			h.stats.Stop()
			return h.waitErr
		}

		if err == nil {
			h.updateProgress(status)
			if status >= 1.0 {
				h.stats.Stop()
				return nil
			}
		}

		select {
		case <-ctx.Done():
			h.waitErr = ctx.Err()
			h.stats.Stop()
			return h.waitErr
		case <-ticker.C:
		}
	}
}

func (h *ServerBackupHandler) updateProgress(status float64) {
	done := uint64(status * serverBackupProgressScale)
	h.progress.Store(done)
	h.stats.ReadRecords.Store(done)
}

// GetStats returns backup statistics derived from the latest server-reported progress.
func (h *ServerBackupHandler) GetStats() *models.BackupStats {
	if h == nil {
		return nil
	}

	return h.stats
}

// GetMetrics returns nil because server-side backup does not expose client-side throughput metrics.
func (h *ServerBackupHandler) GetMetrics() *models.Metrics {
	return nil
}
