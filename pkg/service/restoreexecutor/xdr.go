package restoreexecutor

import (
	"context"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/io/encoding/asbx"
	ioStorage "github.com/aerospike/backup-go/io/storage"
)

func runXDRRestore(
	ctx context.Context,
	client *backup.Client,
	request *model.RestoreRequest,
) (RestoreHandler, error) {
	reader, err := storage.CreateDirReader(
		ctx,
		request.SourceStorage,
		request.BackupDataPath,
		ioStorage.WithValidator(asbx.NewValidator()),
		ioStorage.WithSorting(),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create XDR restore reader: %w", err)
	}

	config := makeXdrRestoreConfig(request)
	config.EncoderType = backup.EncoderTypeASBX
	handler, err := client.Restore(ctx, config, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to start XDR restore: %w", err)
	}

	return handler, nil
}

func makeXdrRestoreConfig(restoreRequest *model.RestoreRequest,
) *backup.ConfigRestore {
	config := backup.NewDefaultRestoreConfig()

	config.RetryPolicy = restoreRequest.Policy.GetRetryPolicyOrDefault()

	config.WritePolicy = makeWritePolicy(restoreRequest)
	config.WritePolicy.RecordExistsAction = as.UPDATE

	if restoreRequest.Policy.Namespace != nil {
		config.Namespace = &backup.RestoreNamespaceConfig{
			Source:      restoreRequest.Policy.Namespace.Source,
			Destination: restoreRequest.Policy.Namespace.Destination,
		}
	}

	config.RecordsPerSecond = util.ValueOrZero(restoreRequest.Policy.Tps)
	config.Bandwidth = int64(util.ValueOrZero(restoreRequest.Policy.Bandwidth))
	config.Parallel = restoreRequest.Policy.GetParallelOrDefault()
	config.MaxAsyncBatches = restoreRequest.Policy.GetMaxAsyncBatchesOrDefault()
	config.BatchSize = restoreRequest.Policy.GetBatchSizeOrDefault()

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

	config.SecretAgentConfig = restoreRequest.SecretAgent.ToSecretAgentConfig()

	return config
}
