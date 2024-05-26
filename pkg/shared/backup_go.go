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
	config.BinList = backupRoutine.BinList
	if backupPolicy.NoRecords != nil && *backupPolicy.NoRecords {
		config.NoRecords = true
	}
	if backupPolicy.NoIndexes != nil && *backupPolicy.NoIndexes {
		config.NoIndexes = true
	}
	if backupPolicy.NoUdfs != nil && *backupPolicy.NoUdfs {
		config.NoUDFs = true
	}

	if len(backupRoutine.SetList) > 0 {
		config.SetList = backupRoutine.SetList
	}

	if backupPolicy.Parallel != nil {
		config.Parallel = int(*backupPolicy.Parallel)
	}

	config.ModBefore = opts.ModBefore
	config.ModAfter = opts.ModAfter

	config.ScanPolicy = a.NewScanPolicy()
	if backupPolicy.MaxRecords != nil {
		config.ScanPolicy.MaxRecords = *backupPolicy.MaxRecords
		config.Parallel = 1
	}

	if backupPolicy.Bandwidth != nil {
		config.Bandwidth = int(*backupPolicy.Bandwidth)
	}

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
		RecordCount: handler.GetStats().GetRecordsTotal(),
		IndexCount:  uint64(handler.GetStats().GetSIndexes()),
		UDFCount:    uint64(handler.GetStats().GetUDFs()),
		ByteCount:   handler.GetStats().GetTotalBytesWritten(),
		FileCount:   handler.GetStats().GetFileCount(),
	}, nil
}

func getWriter(path *string, storage *model.Storage, encoder backup.EncoderFactory) (backup.WriteFactory, error) {
	switch storage.Type {
	case model.Local:
		return backup.NewDirectoryWriterFactory(*path, 0, encoder, true)
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
		}, encoder, true)
	}
	return nil, fmt.Errorf("unknown storage type %v", storage.Type)
}
