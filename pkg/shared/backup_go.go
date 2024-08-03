package shared

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/backup-go/io/aws/s3"

	a "github.com/aerospike/aerospike-client-go/v7"
	"github.com/aerospike/backup-go"
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
func (b *BackupGo) BackupRun(ctx context.Context, backupRoutine *model.BackupRoutine, backupPolicy *model.BackupPolicy,
	client *a.Client, storage *model.Storage, _ *model.SecretAgent,
	timebounds model.TimeBounds, namespace string, path *string,
) (*backup.BackupHandler, error) {
	backupClient, err := backup.NewClient(client, "1", slog.Default())
	if err != nil {
		return nil, fmt.Errorf("failed to create backup client, %w", err)
	}

	config := makeBackupConfig(namespace, backupRoutine, backupPolicy, timebounds)

	writerFactory, err := getWriter(ctx, path, storage)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup writer, %w", err)
	}

	handler, err := backupClient.Backup(ctx, config, writerFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to start backup, %w", err)
	}

	return handler, nil
}

//nolint:funlen
func makeBackupConfig(
	namespace string,
	backupRoutine *model.BackupRoutine,
	backupPolicy *model.BackupPolicy,
	timebounds model.TimeBounds,
) *backup.BackupConfig {
	config := backup.NewDefaultBackupConfig()
	config.Namespace = namespace
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

	if backupPolicy.RecordsPerSecond != nil {
		config.RecordsPerSecond = int(*backupPolicy.RecordsPerSecond)
	}

	config.ModBefore = timebounds.ToTime
	config.ModAfter = timebounds.FromTime

	config.ScanPolicy = a.NewScanPolicy()
	if backupPolicy.MaxRecords != nil {
		config.ScanPolicy.MaxRecords = *backupPolicy.MaxRecords
		config.Parallel = 1
	}
	if backupPolicy.TotalTimeout != nil && *backupPolicy.TotalTimeout > 0 {
		config.ScanPolicy.TotalTimeout = time.Duration(*backupPolicy.TotalTimeout) * time.Millisecond
	}
	if backupPolicy.SocketTimeout != nil && *backupPolicy.SocketTimeout > 0 {
		config.ScanPolicy.SocketTimeout = time.Duration(*backupPolicy.SocketTimeout) * time.Millisecond
	}

	if backupPolicy.Bandwidth != nil {
		config.Bandwidth = int(*backupPolicy.Bandwidth)
	}

	if backupPolicy.CompressionPolicy != nil {
		config.CompressionPolicy = &backup.CompressionPolicy{
			Mode:  backupPolicy.CompressionPolicy.Mode,
			Level: int(backupPolicy.CompressionPolicy.Level),
		}
	}

	if backupPolicy.EncryptionPolicy != nil {
		config.EncryptionPolicy = &backup.EncryptionPolicy{
			Mode:    backupPolicy.EncryptionPolicy.Mode,
			KeyFile: backupPolicy.EncryptionPolicy.KeyFile,
		}
	}

	return config
}

func getWriter(ctx context.Context, path *string, storage *model.Storage) (backup.Writer, error) {
	switch storage.Type {
	case model.Local:
		return backup.NewWriterLocal(*path, true)
	case model.S3:
		bucket, parsedPath, err := util.ParseS3Path(*path)
		if err != nil {
			return nil, err
		}
		return backup.NewWriterS3(ctx, &s3.Config{
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
