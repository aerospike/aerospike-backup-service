package serverrestore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/restoreexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/serverbackup"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
)

const storageTypeS3 = "aws-s3"

// RestoreExecutor starts server-side restores via Aerospike info commands.
type RestoreExecutor struct {
	resolver serverbackup.CredentialsResolver
}

// NewRestoreExecutor builds a server-side restore executor.
func NewRestoreExecutor(resolver serverbackup.CredentialsResolver) *RestoreExecutor {
	return &RestoreExecutor{resolver: resolver}
}

// Run prepares and starts a server-side restore job.
func (r *RestoreExecutor) Run(
	ctx context.Context,
	client aerospike.Restorer,
	request *model.RestoreRequest,
) (restoreexecutor.RestoreHandler, error) {
	infoClient := client.InfoClient()

	namespace, err := destinationNamespace(request)
	if err != nil {
		return nil, err
	}

	jobID := request.BackupDataPath
	if jobID == "" {
		return nil, fmt.Errorf("backup job id is required for server restore")
	}

	config, credentials, err := makeRestoreConfig(ctx, r.resolver, request.SourceStorage)
	if err != nil {
		return nil, err
	}

	if err = infoClient.PrepareServerRestore(ctx, jobID, namespace); err != nil {
		return nil, fmt.Errorf("failed to prepare server restore: %w", err)
	}

	const retryDelay = 5 * time.Second

	for {
		err = infoClient.StartServerRestore(
			ctx,
			jobID,
			namespace,
			config.StorageType,
			config.Bucket,
			config.Region,
			config.Profile,
			credentials.AccessKey,
			credentials.SecretKey,
			config.Endpoint,
		)
		if err == nil {
			break
		}

		if !strings.Contains(err.Error(), "retry once status reports READY") {
			return nil, fmt.Errorf("failed to start server restore: %w", err)
		}

		time.Sleep(retryDelay)
	}

	return newHandler(infoClient, namespace), nil
}

func destinationNamespace(request *model.RestoreRequest) (string, error) {
	if request.Policy.Namespace == nil {
		return "bar", nil
	}

	namespace := ptr.ValueOrZero(request.Policy.Namespace.Destination)
	if namespace == "" {
		namespace = ptr.ValueOrZero(request.Policy.Namespace.Source)
	}
	if namespace == "" {
		return "", fmt.Errorf("destination namespace is required for server restore")
	}

	return namespace, nil
}

type restoreConfig struct {
	StorageType string
	Bucket      string
	Region      string
	Profile     string
	Endpoint    string
}

func makeRestoreConfig(
	ctx context.Context,
	resolver serverbackup.CredentialsResolver,
	storage model.Storage,
) (*restoreConfig, serverbackup.Credentials, error) {
	s3Storage, ok := storage.(*model.S3Storage)
	if !ok {
		return nil, serverbackup.Credentials{}, fmt.Errorf("server restore requires S3 storage, got %T", storage)
	}

	credentials, err := serverbackup.ResolveCredentials(ctx, resolver, storage)
	if err != nil {
		return nil, serverbackup.Credentials{}, err
	}

	return &restoreConfig{
		StorageType: storageTypeS3,
		Bucket:      s3Storage.Bucket,
		Region:      s3Storage.S3Region,
		Profile:     s3Storage.S3Profile,
		Endpoint:    "http://host.docker.internal:9000",
	}, credentials, nil
}

var _ restoreexecutor.Restore = (*RestoreExecutor)(nil)
