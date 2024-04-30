package shared

import (
	"context"
	"fmt"
	a "github.com/aerospike/aerospike-client-go/v7"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup/pkg/model"
	"log/slog"
	"net/url"
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
		return nil, err
	}

	backupClient, err := backup.NewClient(client, "1", slog.Default(), backup.NewConfig())
	if err != nil {
		return nil, err
	}

	config := backup.NewRestoreConfig()

	reader, err := getReader(restoreRequest.Dir, restoreRequest.SourceStorage, config.DecoderFactory)
	if err != nil {
		return nil, err
	}

	handler, err := backupClient.Restore(context.TODO(), config, reader)
	if err != nil {
		return nil, err
	}

	err = handler.Wait(context.TODO())
	if err != nil {
		return nil, err
	}

	stats := handler.GetStats()
	return &model.RestoreResult{
		TotalRecords: stats.GetRecords(),
		IndexCount:   uint64(stats.GetSIndexes()),
		UDFCount:     uint64(stats.GetUDFs()),
	}, nil
}

func getReader(path *string, storage *model.Storage, decoder backup.DecoderFactory) (backup.ReaderFactory, error) {
	switch storage.Type {
	case model.Local:
		return backup.NewDirectoryReaderFactory(*path, decoder), nil
	case model.S3:
		parsed, err := url.Parse(*path)
		if err != nil {
			return nil, err
		}
		return backup.NewS3ReaderFactory(&backup.S3Config{
			Bucket:    parsed.Host,
			Region:    *storage.S3Region,
			Endpoint:  *storage.S3EndpointOverride,
			Profile:   *storage.S3Profile,
			Prefix:    parsed.Path,
			ChunkSize: 0,
		}, nil)
	}
	return nil, fmt.Errorf("unknown storage type %v", storage.Type)
}
