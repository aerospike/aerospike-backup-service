package serverbackup

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go/models"
	"github.com/aerospike/backup-go/pkg/asinfo"
	infoModels "github.com/aerospike/backup-go/pkg/asinfo/models"
)

const (
	storageTypeS3 = "aws-s3"
	// progressScale matches backup-go GetBackupStatus normalization.
	progressScale = 4096
	pollInterval  = time.Second
)

// Credentials holds resolved object-storage credentials for server-side backup.
type Credentials struct {
	AccessKey string
	SecretKey string
}

// CredentialsResolver resolves secret values for server-side backup.
type CredentialsResolver interface {
	// Resolve returns the plaintext value for a secret reference.
	Resolve(ctx context.Context, agent *model.SecretAgent, value string) (string, error)
}

// Handler monitors a server-side backup job started via Aerospike info commands.
type Handler struct {
	infoClient infoClient
	jobID      string
	stats      *models.BackupStats
	progress   atomic.Uint64
	waitErr    error
}

var _ backupHandler = (*Handler)(nil)

type backupHandler interface {
	Wait(context.Context) error
	GetStats() *models.BackupStats
	GetMetrics() *models.Metrics
}

type infoClient interface {
	StartServerBackup(
		ctx context.Context,
		backup *infoModels.RequestBackup,
	) (string, error)
	GetBackupStatus(ctx context.Context) (float64, error)
}

// Run starts a server-side backup and returns a handler for monitoring progress.
func Run(
	ctx context.Context,
	client infoClient,
	namespace string,
	routine *model.BackupRoutine,
	credentials Credentials,
	spec model.BackupRunSpec,
) (*Handler, error) {
	backupRequest, err := makeBackupRequest(namespace, routine, credentials, spec)
	if err != nil {
		return nil, fmt.Errorf("failed to make server backup request: %w", err)
	}

	jobID, err := client.StartServerBackup(ctx, backupRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to start server backup: %w", err)
	}

	return &Handler{
		infoClient: client,
		jobID:      jobID,
		stats:      models.NewBackupStats(),
	}, nil
}

func makeBackupRequest(
	namespace string,
	routine *model.BackupRoutine,
	credentials Credentials,
	spec model.BackupRunSpec,
) (*infoModels.RequestBackup, error) {
	s3Storage, ok := routine.Storage.(*model.S3Storage)
	if !ok {
		return nil, fmt.Errorf("server backup requires S3 storage, got %T", routine.Storage)
	}

	var from, to string
	if spec.TimeBounds.FromTime != nil {
		from = strconv.FormatInt(spec.TimeBounds.FromTime.Unix(), 10)
	}
	if spec.TimeBounds.ToTime != nil {
		to = strconv.FormatInt(spec.TimeBounds.ToTime.Unix(), 10)
	}

	return &infoModels.RequestBackup{
		RequestCommon: infoModels.RequestCommon{
			Namespace: namespace,
			Storage:   storageTypeS3,
			Bucket:    s3Storage.Bucket,
			Region:    s3Storage.S3Region,
			Profile:   s3Storage.S3Profile,
			AccessKey: credentials.AccessKey,
			SecretKey: credentials.SecretKey,
			Endpoint:  "http://host.docker.internal:9000",
		},
		ModifiedBefore:     to,
		ModifiedAfter:      from,
		SetList:            strings.Join(routine.SetList, ","),
		NoIndexes:          false,
		NoUDFs:             false,
		EnableChangeStream: false,
	}, nil
}

// ResolveCredentials resolves S3 credentials from storage configuration.
func ResolveCredentials(
	ctx context.Context,
	resolver CredentialsResolver,
	storage model.Storage,
) (Credentials, error) {
	s3Storage, ok := storage.(*model.S3Storage)
	if !ok {
		return Credentials{}, fmt.Errorf("server backup requires S3 storage, got %T", storage)
	}

	if s3Storage.Auth == nil {
		return Credentials{}, nil
	}

	accessKey, err := resolver.Resolve(ctx, s3Storage.Auth.SecretAgent, s3Storage.Auth.KeyIDSecret)
	if err != nil {
		return Credentials{}, fmt.Errorf("failed to resolve access key ID: %w", err)
	}

	secretKey, err := resolver.Resolve(ctx, s3Storage.Auth.SecretAgent, s3Storage.Auth.AccessKeySecret)
	if err != nil {
		return Credentials{}, fmt.Errorf("failed to resolve secret access key: %w", err)
	}

	return Credentials{
		AccessKey: accessKey,
		SecretKey: secretKey,
	}, nil
}

// Wait polls backup status until the job completes or the context is canceled.
func (h *Handler) Wait(ctx context.Context) error {
	if h == nil {
		return nil
	}

	ticker := time.NewTicker(pollInterval)
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

func (h *Handler) updateProgress(status float64) {
	if h.stats.StartTime.IsZero() {
		h.stats.Start()
	}
	h.stats.TotalRecords.Store(progressScale)

	done := uint64(status * progressScale)
	h.progress.Store(done)
	h.stats.ReadRecords.Store(done)
}

// GetStats returns backup statistics derived from the latest server-reported progress.
func (h *Handler) GetStats() *models.BackupStats {
	if h == nil {
		return nil
	}

	return h.stats
}

// GetMetrics returns nil because server-side backup does not expose client-side throughput metrics.
func (h *Handler) GetMetrics() *models.Metrics {
	return nil
}
