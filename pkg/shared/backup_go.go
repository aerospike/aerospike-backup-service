package shared

import (
	"context"
	"fmt"
	"log/slog"

	a "github.com/aerospike/aerospike-client-go/v7"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/io/local"
	"github.com/aerospike/backup-go/io/s3"
	"github.com/aerospike/backup-go/models"
	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/util"
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
	cluster *model.AerospikeCluster, storage *model.Storage, _ *model.SecretAgent,
	opts BackupOptions, namespace *string, path *string) (*BackupStat, error) {

	var err error
	client, err := a.NewClientWithPolicyAndHost(cluster.ASClientPolicy(), cluster.ASClientHosts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to aerospike cluster, %w", err)
	}
	defer client.Close()

	backupClient, err := backup.NewClient(client, "1", slog.Default())
	if err != nil {
		return nil, fmt.Errorf("failed to create backup client, %w", err)
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

	if backupPolicy.FileLimit != nil {
		config.FileLimit = *backupPolicy.FileLimit * 1_048_576 // lib expects limit in bytes.
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

	if backupPolicy.CompressionPolicy != nil {
		config.CompressionPolicy = &models.CompressionPolicy{
			Mode:  backupPolicy.CompressionPolicy.Mode,
			Level: int(backupPolicy.CompressionPolicy.Level),
		}
	}

	if backupPolicy.EncryptionPolicy != nil {
		config.EncryptionPolicy = &models.EncryptionPolicy{
			Mode:    backupPolicy.EncryptionPolicy.Mode,
			KeyFile: backupPolicy.EncryptionPolicy.KeyFile,
		}
	}

	writerFactory, err := getWriter(path, storage)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup writer, %w", err)
	}

	handler, err := backupClient.Backup(context.TODO(), config, writerFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to start backup, %w", err)
	}

	err = handler.Wait(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("error during backup, %w", err)
	}

	return &BackupStat{
		RecordCount: handler.GetStats().GetReadRecords(),
		IndexCount:  uint64(handler.GetStats().GetSIndexes()),
		UDFCount:    uint64(handler.GetStats().GetUDFs()),
		ByteCount:   handler.GetStats().GetBytesWritten(),
		FileCount:   handler.GetStats().GetFileCount(),
	}, nil
}

func getWriter(path *string, storage *model.Storage) (backup.WriteFactory, error) {
	switch storage.Type {
	case model.Local:
		return local.NewDirectoryWriterFactory(*path, true)
	case model.S3:
		bucket, parsedPath, err := util.ParseS3Path(*path)
		if err != nil {
			return nil, err
		}
		return s3.NewS3WriterFactory(&s3.StorageConfig{
			Bucket:    bucket,
			Region:    *storage.S3Region,
			Endpoint:  *storage.S3EndpointOverride,
			Profile:   *storage.S3Profile,
			Prefix:    parsedPath,
			ChunkSize: 0,
		}, true)
	}
	return nil, fmt.Errorf("unknown storage type %v", storage.Type)
}
