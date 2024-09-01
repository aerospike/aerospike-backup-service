package service

import (
	"context"
	"fmt"
	"github.com/aerospike/aerospike-backup-service/pkg/model"
	"time"

	"github.com/aerospike/aerospike-backup-service/pkg/util"
	a "github.com/aerospike/aerospike-client-go/v7"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/io/aws/s3"
)

// RestoreGo implements the [Restore] interface.
type RestoreGo struct {
}

// NewRestoreGo returns a new RestoreGo instance.
func NewRestoreGo() *RestoreGo {
	return &RestoreGo{}
}

// RestoreRun creates a [backup.Client] and initiates the restore operation.
// A restore handler is returned to monitor the job status.
func (r *RestoreGo) RestoreRun(
	ctx context.Context,
	client *backup.Client,
	restoreRequest *model.RestoreRequestInternal,
) (RestoreHandler, error) {
	var err error

	config := makeRestoreConfig(restoreRequest)

	reader, err := getReader(ctx, restoreRequest.Dir, restoreRequest.SourceStorage)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup reader, %w", err)
	}

	handler, err := client.Restore(ctx, config, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to start restore, %w", err)
	}

	return handler, nil
}

//nolint:funlen
func makeRestoreConfig(restoreRequest *model.RestoreRequestInternal,
) *backup.RestoreConfig {
	config := backup.NewDefaultRestoreConfig()
	config.BinList = restoreRequest.Policy.BinList
	config.SetList = restoreRequest.Policy.SetList

	config.RetryPolicy = util.Ptr(restoreRequest.Policy.RetryPolicy)

	if restoreRequest.Policy.Tps != nil {
		config.RecordsPerSecond = int(*restoreRequest.Policy.Tps)
	}
	if restoreRequest.Policy.Bandwidth != nil {
		config.Bandwidth = int(*restoreRequest.Policy.Bandwidth)
	}

	config.WritePolicy = makeWritePolicy(restoreRequest)
	if restoreRequest.Policy.NoRecords != nil && *restoreRequest.Policy.NoRecords {
		config.NoRecords = true
	}
	if restoreRequest.Policy.NoIndexes != nil && *restoreRequest.Policy.NoIndexes {
		config.NoIndexes = true
	}
	if restoreRequest.Policy.NoUdfs != nil && *restoreRequest.Policy.NoUdfs {
		config.NoUDFs = true
	}

	if restoreRequest.Policy.Namespace != nil {
		config.Namespace = &backup.RestoreNamespaceConfig{
			Source:      restoreRequest.Policy.Namespace.Source,
			Destination: restoreRequest.Policy.Namespace.Destination,
		}
	}

	if restoreRequest.Policy.Parallel != nil {
		config.Parallel = int(*restoreRequest.Policy.Parallel)
	}
	if restoreRequest.Policy.MaxAsyncBatches != nil {
		config.MaxAsyncBatches = int(*restoreRequest.Policy.MaxAsyncBatches)
	}
	if restoreRequest.Policy.BatchSize != nil {
		config.BatchSize = int(*restoreRequest.Policy.BatchSize)
	}
	if restoreRequest.Policy.DisableBatchWrites != nil {
		config.DisableBatchWrites = *restoreRequest.Policy.DisableBatchWrites
	}
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

	if restoreRequest.SecretAgent != nil {
		config.SecretAgentConfig = &backup.SecretAgentConfig{
			ConnectionType:     restoreRequest.SecretAgent.ConnectionType,
			Address:            restoreRequest.SecretAgent.Address,
			Port:               restoreRequest.SecretAgent.Port,
			TimeoutMillisecond: restoreRequest.SecretAgent.Timeout,
			CaFile:             restoreRequest.SecretAgent.TLSCAString,
			IsBase64:           restoreRequest.SecretAgent.IsBase64,
		}
	}
	return config
}

func makeWritePolicy(restoreRequest *model.RestoreRequestInternal) *a.WritePolicy {
	writePolicy := a.NewWritePolicy(0, 0)
	writePolicy.GenerationPolicy = a.EXPECT_GEN_GT
	if restoreRequest.Policy.NoGeneration != nil && *restoreRequest.Policy.NoGeneration {
		writePolicy.GenerationPolicy = a.NONE
	}

	// Invalid options: --unique is mutually exclusive with --replace and --no-generation.
	writePolicy.RecordExistsAction = recordExistsAction(
		restoreRequest.Policy.Replace, restoreRequest.Policy.Unique)

	if restoreRequest.Policy.Timeout != nil && *restoreRequest.Policy.Timeout > 0 {
		writePolicy.TotalTimeout = time.Duration(*restoreRequest.Policy.Timeout) *
			time.Millisecond
	}

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

// getReader instantiates and returns a reader for the restore operation
// according to the specified storage type.
func getReader(ctx context.Context, path *string, storage *model.Storage,
) (backup.StreamingReader, error) {
	switch storage.Type {
	case model.Local:
		return backup.NewStreamingReaderLocal(*path, backup.EncoderTypeASB)
	case model.S3:
		bucket, parsedPath, err := util.ParseS3Path(*path)
		if err != nil {
			return nil, err
		}
		return backup.NewStreamingReaderS3(ctx, &s3.Config{
			Bucket:          bucket,
			Region:          *storage.S3Region,
			Endpoint:        *storage.S3EndpointOverride,
			Profile:         *storage.S3Profile,
			Prefix:          parsedPath,
			MaxConnsPerHost: storage.MaxConnsPerHost,
			MinPartSize:     storage.MinPartSize,
		}, backup.EncoderTypeASB)
	}
	return nil, fmt.Errorf("unknown storage type %v", storage.Type)
}
