package shared

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	a "github.com/aerospike/aerospike-client-go/v7"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/util"
)

// RestoreGo implements the Restore interface.
type RestoreGo struct {
}

var _ Restore = (*RestoreGo)(nil)

// NewRestoreGo returns a new RestoreGo instance.
func NewRestoreGo() *RestoreGo {
	return &RestoreGo{}
}

// RestoreRun calls the restore function from the asbackup library.
func (r *RestoreGo) RestoreRun(ctx context.Context, client *a.Client, restoreRequest *model.RestoreRequestInternal,
) (*model.RestoreResult, error) {
	var err error
	backupClient, err := backup.NewClient(client, "1", slog.Default())
	if err != nil {
		return nil, fmt.Errorf("failed to create backup client, %w", err)
	}

	config := makeRestoreConfig(restoreRequest, client)

	reader, err := getReader(ctx, restoreRequest.Dir, restoreRequest.SourceStorage)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup reader, %w", err)
	}

	handler, err := backupClient.Restore(ctx, config, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to start restore, %w", err)
	}

	err = handler.Wait(ctx)
	if err != nil {
		return nil, fmt.Errorf("error during restore, %w", err)
	}

	stats := handler.GetStats()
	return &model.RestoreResult{
		TotalRecords:    stats.GetReadRecords(),
		InsertedRecords: stats.GetRecordsInserted(),
		IndexCount:      uint64(stats.GetSIndexes()),
		UDFCount:        uint64(stats.GetUDFs()),
		FresherRecords:  stats.GetRecordsFresher(),
		SkippedRecords:  stats.GetRecordsSkipped(),
		ExistedRecords:  stats.GetRecordsExisted(),
		ExpiredRecords:  stats.GetRecordsExpired(),
		TotalBytes:      stats.GetTotalBytesRead(),
	}, nil
}

//nolint:funlen
func makeRestoreConfig(restoreRequest *model.RestoreRequestInternal, client *a.Client) *backup.RestoreConfig {
	config := backup.NewRestoreConfig()
	config.BinList = restoreRequest.Policy.BinList
	config.SetList = restoreRequest.Policy.SetList
	config.WritePolicy = client.DefaultWritePolicy
	config.WritePolicy.MaxRetries = 100
	if restoreRequest.Policy.Tps != nil {
		config.RecordsPerSecond = int(*restoreRequest.Policy.Tps)
	}
	if restoreRequest.Policy.Bandwidth != nil {
		config.Bandwidth = int(*restoreRequest.Policy.Bandwidth)
	}

	config.WritePolicy.GenerationPolicy = a.EXPECT_GEN_GT
	if restoreRequest.Policy.NoGeneration != nil && *restoreRequest.Policy.NoGeneration {
		config.WritePolicy.GenerationPolicy = a.NONE
	}

	// Invalid options: --unique is mutually exclusive with --replace and --no-generation.
	config.WritePolicy.RecordExistsAction = recordExistsAction(restoreRequest.Policy.Replace, restoreRequest.Policy.Unique)

	if restoreRequest.Policy.Timeout != nil && *restoreRequest.Policy.Timeout > 0 {
		config.WritePolicy.TotalTimeout = time.Duration(*restoreRequest.Policy.Timeout) * time.Millisecond
	}
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
		config.Namespace = &models.RestoreNamespace{
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
		config.CompressionPolicy = &models.CompressionPolicy{
			Mode:  restoreRequest.Policy.CompressionPolicy.Mode,
			Level: int(restoreRequest.Policy.CompressionPolicy.Level),
		}
	}
	if restoreRequest.Policy.EncryptionPolicy != nil {
		config.EncryptionPolicy = &models.EncryptionPolicy{
			Mode:    restoreRequest.Policy.EncryptionPolicy.Mode,
			KeyFile: restoreRequest.Policy.EncryptionPolicy.KeyFile,
		}
	}
	return config
}

func recordExistsAction(replace, unique *bool) a.RecordExistsAction {
	switch {
	case replace != nil && *replace && unique != nil && *unique:
		panic("Replace and Unique options are contradictory")

	// overwrite all bins of an existing record
	case replace != nil && *replace:
		return a.REPLACE

	// only insert the record if it does not already exist in the database
	case unique != nil && *unique:
		return a.CREATE_ONLY

	// default behaviour: merge bins with existing record, or create a new record if it does not exist
	default:
		return a.UPDATE
	}
}

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
		return backup.NewStreamingReaderS3(ctx, &models.S3Config{
			Bucket:    bucket,
			Region:    *storage.S3Region,
			Endpoint:  *storage.S3EndpointOverride,
			Profile:   *storage.S3Profile,
			Prefix:    parsedPath,
			ChunkSize: 0,
		}, backup.EncoderTypeASB)
	}
	return nil, fmt.Errorf("unknown storage type %v", storage.Type)
}
