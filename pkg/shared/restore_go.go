package shared

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"

	a "github.com/aerospike/aerospike-client-go/v7"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/encoding"
	"github.com/aerospike/backup-go/io/local"
	"github.com/aerospike/backup-go/io/s3"
	"github.com/aerospike/backup/pkg/model"
)

// RestoreGo implements the Restore interface.
type RestoreGo struct {
}

var _ Restore = (*RestoreGo)(nil)

// NewRestoreGo returns a new RestoreGo instance.
func NewRestoreGo() *RestoreGo {
	return &RestoreGo{}
}

// RestoreRun calls the restore_run function from the asrestore shared library.
//
//nolint:funlen,gocritic
func (r *RestoreGo) RestoreRun(restoreRequest *model.RestoreRequestInternal) (*model.RestoreResult, error) {
	var err error
	client, err := a.NewClientWithPolicyAndHost(
		restoreRequest.DestinationCuster.ASClientPolicy(),
		restoreRequest.DestinationCuster.ASClientHosts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to aerospike cluster, %w", err)
	}
	defer client.Close()

	backupClient, err := backup.NewClient(client, "1", slog.Default(), backup.NewConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create backup client, %w", err)
	}

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

	if restoreRequest.Policy.NoRecords != nil && *restoreRequest.Policy.NoRecords {
		config.NoRecords = true
	}
	if restoreRequest.Policy.NoIndexes != nil && *restoreRequest.Policy.NoIndexes {
		config.NoIndexes = true
	}
	if restoreRequest.Policy.NoUdfs != nil && *restoreRequest.Policy.NoUdfs {
		config.NoUDFs = true
	}

	config.Namespace = restoreRequest.Policy.Namespace

	reader, err := getReader(restoreRequest.Dir, restoreRequest.SourceStorage, config.DecoderFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup reader, %w", err)
	}

	handler, err := backupClient.Restore(context.TODO(), config, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to start restore, %w", err)
	}

	err = handler.Wait(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("error during restore, %w", err)
	}

	stats := handler.GetStats()
	return &model.RestoreResult{
		TotalRecords:    stats.GetRecordsTotal(),
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

func getReader(path *string, storage *model.Storage, decoder encoding.DecoderFactory) (backup.ReaderFactory, error) {
	switch storage.Type {
	case model.Local:
		return local.NewDirectoryReaderFactory(*path, decoder)
	case model.S3:
		parsed, err := url.Parse(*path)
		if err != nil {
			return nil, err
		}
		return s3.NewS3ReaderFactory(&s3.StorageConfig{
			Bucket:    parsed.Host,
			Region:    *storage.S3Region,
			Endpoint:  *storage.S3EndpointOverride,
			Profile:   *storage.S3Profile,
			Prefix:    parsed.Path,
			ChunkSize: 0,
		}, decoder)
	}
	return nil, fmt.Errorf("unknown storage type %v", storage.Type)
}
