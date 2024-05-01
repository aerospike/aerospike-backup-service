package shared

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"

	a "github.com/aerospike/aerospike-client-go/v7"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup/pkg/model"
)

// BackupGo implements the Backup interface.
type BackupGo struct {
}

var _ Backup = (*BackupGo)(nil)

// NewBackupGo returns a new BackupGo instance.
func NewBackupGo() *BackupGo {
	return &BackupGo{}
}

// BackupRun calls the backup_run function from the asbackup shared library.
//
//nolint:funlen,gocritic
func (b *BackupGo) BackupRun(backupRoutine *model.BackupRoutine, backupPolicy *model.BackupPolicy,
	cluster *model.AerospikeCluster, storage *model.Storage, secretAgent *model.SecretAgent,
	opts BackupOptions, namespace *string, path *string) (*BackupStat, error) {

	var err error
	client, err := a.NewClientWithPolicyAndHost(cluster.ASClientPolicy(), cluster.ASClientHosts()...)
	if err != nil {
		return nil, err
	}

	backupClient, err := backup.NewClient(client, "1", slog.Default(), backup.NewConfig())
	if err != nil {
		return nil, err
	}

	config := backup.NewBackupConfig()
	config.Namespace = *namespace

	writerFactory, err := getWriter(path, storage, config.EncoderFactory)
	if err != nil {
		return nil, err
	}

	handler, err := backupClient.Backup(context.TODO(), config, writerFactory)
	if err != nil {
		return nil, err
	}

	err = handler.Wait(context.TODO())
	if err != nil {
		return nil, err
	}

	return &BackupStat{
		RecordCount: handler.GetStats().GetRecords(),
		IndexCount:  uint64(handler.GetStats().GetSIndexes()),
		UDFCount:    uint64(handler.GetStats().GetUDFs()),
		ByteCount:   1,
	}, nil
}

func getWriter(path *string, storage *model.Storage, encoder backup.EncoderFactory) (backup.WriteFactory, error) {
	switch storage.Type {
	case model.Local:
		return backup.NewDirectoryWriterFactory(*path, 0, encoder)
	case model.S3:
		parsed, err := url.Parse(*path)
		if err != nil {
			return nil, err
		}
		return backup.NewS3WriterFactory(&backup.S3Config{
			Bucket:    parsed.Host,
			Region:    *storage.S3Region,
			Endpoint:  *storage.S3EndpointOverride,
			Profile:   *storage.S3Profile,
			Prefix:    parsed.Path,
			ChunkSize: 0,
		}, encoder)
	}
	return nil, fmt.Errorf("unknown storage type %v", storage.Type)
}
