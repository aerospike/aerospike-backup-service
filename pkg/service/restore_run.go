package service

import (
	"context"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	a "github.com/aerospike/aerospike-client-go/v7"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/io/encoding/asb"
)

// RestoreRunner implements the [Restore] interface.
type RestoreRunner struct {
}

// NewRestore returns a new RestoreRunner instance.
func NewRestore() *RestoreRunner {
	return &RestoreRunner{}
}

// Run initiates the restore operation.
// A restore handler is returned to monitor the job status.
func (r *RestoreRunner) Run(
	ctx context.Context,
	client *backup.Client,
	request *model.RestoreRequest,
) (RestoreHandler, error) {
	var err error

	config := makeRestoreConfig(request)

	reader, err := storage.CreateReader(ctx, request.SourceStorage, request.BackupDataPath, false, asb.NewValidator(), "")
	if err != nil {
		return nil, fmt.Errorf("failed to create backup reader, %w", err)
	}

	handler, err := client.Restore(ctx, config, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to start restore, %w", err)
	}

	return handler, nil
}

func makeRestoreConfig(restoreRequest *model.RestoreRequest,
) *backup.RestoreConfig {
	config := backup.NewDefaultRestoreConfig()
	config.BinList = restoreRequest.Policy.BinList
	config.SetList = restoreRequest.Policy.SetList

	config.RetryPolicy = restoreRequest.Policy.GetRetryPolicyOrDefault()

	config.WritePolicy = makeWritePolicy(restoreRequest)
	config.NoRecords = util.ValueOrZero(restoreRequest.Policy.NoRecords)
	config.NoIndexes = util.ValueOrZero(restoreRequest.Policy.NoIndexes)
	config.NoUDFs = util.ValueOrZero(restoreRequest.Policy.NoUdfs)

	if restoreRequest.Policy.Namespace != nil {
		config.Namespace = &backup.RestoreNamespaceConfig{
			Source:      restoreRequest.Policy.Namespace.Source,
			Destination: restoreRequest.Policy.Namespace.Destination,
		}
	}

	config.RecordsPerSecond = util.ValueOrZero(restoreRequest.Policy.Tps)
	config.Bandwidth = util.ValueOrZero(restoreRequest.Policy.Bandwidth)
	config.Parallel = restoreRequest.Policy.GetParallelOrDefault()
	config.MaxAsyncBatches = restoreRequest.Policy.GetMaxAsyncBatchesOrDefault()
	config.BatchSize = restoreRequest.Policy.GetBatchSizeOrDefault()
	config.DisableBatchWrites = util.ValueOrZero(restoreRequest.Policy.DisableBatchWrites)

	if restoreRequest.Policy.CompressionPolicy != nil {
		config.CompressionPolicy = &backup.CompressionPolicy{
			Mode:  restoreRequest.Policy.CompressionPolicy.Mode,
			Level: int(restoreRequest.Policy.CompressionPolicy.Level),
		}
	}
	if restoreRequest.Policy.EncryptionPolicy != nil {
		config.EncryptionPolicy = &backup.EncryptionPolicy{
			Mode:      restoreRequest.Policy.EncryptionPolicy.Mode,
			KeyFile:   restoreRequest.Policy.EncryptionPolicy.KeyFile,
			KeySecret: restoreRequest.Policy.EncryptionPolicy.KeySecret,
			KeyEnv:    restoreRequest.Policy.EncryptionPolicy.KeyEnv,
		}
	}

	config.ExtraTTL = util.ValueOrZero(restoreRequest.Policy.ExtraTTL)

	config.SecretAgentConfig = restoreRequest.SecretAgent.ToSecretAgentConfig()

	return config
}

func makeWritePolicy(restoreRequest *model.RestoreRequest) *a.WritePolicy {
	writePolicy := a.NewWritePolicy(0, 0)
	writePolicy.GenerationPolicy = a.EXPECT_GEN_GT
	writePolicy.SendKey = true
	if restoreRequest.Policy.NoGeneration != nil && *restoreRequest.Policy.NoGeneration {
		writePolicy.GenerationPolicy = a.NONE
	}

	// Invalid options: --unique is mutually exclusive with --replace and --no-generation.
	writePolicy.RecordExistsAction = recordExistsAction(
		restoreRequest.Policy.Replace, restoreRequest.Policy.Unique)

	if restoreRequest.Policy.SocketTimeout != nil {
		writePolicy.SocketTimeout = *restoreRequest.Policy.SocketTimeout
	}
	if restoreRequest.Policy.TotalTimeout != nil {
		writePolicy.TotalTimeout = *restoreRequest.Policy.TotalTimeout
	}
	writePolicy.MaxRetries = int(restoreRequest.Policy.GetRetryPolicyOrDefault().MaxRetries)

	return writePolicy
}

func recordExistsAction(replace, unique *bool) a.RecordExistsAction {
	switch {
	// overwrite all bins of an existing record
	case replace != nil && *replace:
		return a.REPLACE

	// only insert the record if it does not already exist in the database
	case unique != nil && *unique:
		return a.CREATE_ONLY

	// default behaviour: merge bins with existing record, or create a new
	// record if it does not exist
	default:
		return a.UPDATE
	}
}
