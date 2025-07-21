package restoreexecutor

import (
	"context"
	"fmt"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/io/encoding/asb"
	ioStorage "github.com/aerospike/backup-go/io/storage"
)

const megabyte = 1_048_576

func runScanRestore(ctx context.Context, client *backup.Client, request *model.RestoreRequest) (RestoreHandler, error) {
	reader, err := storage.CreateDirReader(ctx,
		request.SourceStorage, request.BackupDataPath, ioStorage.WithValidator(asb.NewValidator()))
	if err != nil {
		return nil, fmt.Errorf("failed to create backup reader, %w", err)
	}

	config := makeRestoreConfig(request)
	handler, err := client.Restore(ctx, config, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to start restore, %w", err)
	}

	return handler, nil
}

func makeRestoreConfig(restoreRequest *model.RestoreRequest,
) *backup.ConfigRestore {
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
	config.Bandwidth = util.ValueOrZero(restoreRequest.Policy.Bandwidth) * megabyte
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
	config.MetricsEnabled = true

	return config
}

func makeWritePolicy(restoreRequest *model.RestoreRequest) *as.WritePolicy {
	writePolicy := as.NewWritePolicy(0, 0)
	writePolicy.GenerationPolicy = as.EXPECT_GEN_GT
	writePolicy.SendKey = true
	if restoreRequest.Policy.NoGeneration != nil && *restoreRequest.Policy.NoGeneration {
		writePolicy.GenerationPolicy = as.NONE
	}

	// Invalid options: --unique is mutually exclusive with --replace and --no-generation.
	writePolicy.RecordExistsAction = recordExistsAction(
		restoreRequest.Policy.Replace, restoreRequest.Policy.Unique)

	writePolicy.SocketTimeout = calculateSocketTimeout(restoreRequest.Policy)

	if restoreRequest.Policy.TotalTimeout != nil {
		writePolicy.TotalTimeout = *restoreRequest.Policy.TotalTimeout
	}
	writePolicy.MaxRetries = int(restoreRequest.Policy.GetRetryPolicyOrDefault().MaxRetries)

	return writePolicy
}

func recordExistsAction(replace, unique *bool) as.RecordExistsAction {
	switch {
	// overwrite all bins of an existing record
	case replace != nil && *replace:
		return as.REPLACE

	// only insert the record if it does not already exist in the database
	case unique != nil && *unique:
		return as.CREATE_ONLY

	// default behaviour: merge bins with existing record, or create a new
	// record if it does not exist
	default:
		return as.UPDATE
	}
}

func calculateSocketTimeout(policy *model.RestorePolicy) time.Duration {
	if policy.SocketTimeout != nil {
		return *policy.SocketTimeout
	}

	return model.DefaultSocketTimeout
}
