package serverrestore

import (
	"context"
	"errors"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/restoreexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/serverbackup"
	"github.com/aerospike/backup-go"
)

// FuzzyRestoreExecutor starts server-side restores via Aerospike info commands.
type FuzzyRestoreExecutor struct {
	resolver serverbackup.CredentialsResolver
}

var _ restoreexecutor.Restore = (*FuzzyRestoreExecutor)(nil)

// NewFuzzyRestoreExecutor builds a server-side restore executor.
func NewFuzzyRestoreExecutor(resolver serverbackup.CredentialsResolver) *FuzzyRestoreExecutor {
	return &FuzzyRestoreExecutor{resolver: resolver}
}

// Run prepares and starts a server-side restore job.
func (r *FuzzyRestoreExecutor) Run(
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
		return nil, errors.New("backup job id is required for server fuzzy restore")
	}

	restoreRequest, err := makeRestoreRequest(ctx, r.resolver, request.SourceStorage, namespace, jobID)
	if err != nil {
		return nil, err
	}
	restoreRequest.FuzzyRestore = true

	err = infoClient.StartServerRestore(ctx, restoreRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to start server fuzzy restore: %w", err)
	}

	return newHandler(infoClient, namespace), nil
}
