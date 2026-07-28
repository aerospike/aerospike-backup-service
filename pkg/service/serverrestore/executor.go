package serverrestore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/restoreexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/serverbackup"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/pkg/asinfo"
	infoModels "github.com/aerospike/backup-go/pkg/asinfo/models"
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
	var infoClient backup.ServerBackupInfo = client.InfoClient()

	namespace, err := destinationNamespace(request)
	if err != nil {
		return nil, err
	}

	jobID := request.BackupDataPath
	if jobID == "" {
		return nil, errors.New("backup job id is required for server restore")
	}

	if err = infoClient.PrepareServerRestore(ctx, jobID, namespace); err != nil {
		return nil, fmt.Errorf("failed to prepare server restore: %w", err)
	}

	const retryDelay = 5 * time.Second
	for {
		status, err := infoClient.GetRestoreStatus(ctx, namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to get status of server restore: %w", err)
		}

		if status == asinfo.RestoreStateFailed {
			return nil, errors.New("server restore failed")
		}

		if status == asinfo.RestoreStateReady {
			break
		}

		time.Sleep(retryDelay)
	}

	restoreRequest, err := makeRestoreRequest(ctx, r.resolver, request.SourceStorage, namespace, jobID)
	if err != nil {
		return nil, err
	}

	err = infoClient.StartServerRestore(ctx, restoreRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to start server restore: %w", err)
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
		return "", errors.New("destination namespace is required for server restore")
	}

	return namespace, nil
}

func makeRestoreRequest(
	ctx context.Context,
	resolver serverbackup.CredentialsResolver,
	storage model.Storage,
	namespace, jobID string,
) (*infoModels.RequestRestore, error) {
	s3Storage, ok := storage.(*model.S3Storage)
	if !ok {
		return nil, fmt.Errorf("server restore requires S3 storage, got %T", storage)
	}

	credentials, err := serverbackup.ResolveCredentials(ctx, resolver, storage)
	if err != nil {
		return nil, err
	}

	return &infoModels.RequestRestore{
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
		JobID: jobID,
	}, nil
}

var _ restoreexecutor.Restore = (*RestoreExecutor)(nil)
