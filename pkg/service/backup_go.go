package service

import (
	"context"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/service/storage"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/util"
	a "github.com/aerospike/aerospike-client-go/v7"
	"github.com/aerospike/backup-go"
)

// BackupGo implements the [Backup] interface.
type BackupGo struct {
}

// NewBackupGo returns a new BackupGo instance.
func NewBackupGo() *BackupGo {
	return &BackupGo{}
}

// BackupRun creates a [backup.Client] and initiates the backup operation.
// A backup handler is returned to monitor the job status.
func (b *BackupGo) BackupRun(
	ctx context.Context,
	backupRoutine *model.BackupRoutine,
	backupPolicy *model.BackupPolicy,
	client *backup.Client,
	s model.Storage,
	secretAgent *model.SecretAgent,
	timebounds model.TimeBounds,
	namespace string,
	path string,
) (BackupHandler, error) {
	config := makeBackupConfig(namespace, backupRoutine, backupPolicy, timebounds, secretAgent)

	writerFactory, err := storage.CreateWriter(ctx, s, path, false,
		backupPolicy.RemoveFiles.RemoveFullBackup(), false)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup writer, %w", err)
	}

	handler, err := client.Backup(ctx, config, writerFactory, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start backup, %w", err)
	}

	return handler, nil
}

func makeBackupConfig(
	namespace string,
	backupRoutine *model.BackupRoutine,
	backupPolicy *model.BackupPolicy,
	timebounds model.TimeBounds,
	secretAgent *model.SecretAgent,
) *backup.BackupConfig {
	config := backup.NewDefaultBackupConfig()

	config.Namespace = namespace
	config.BinList = backupRoutine.BinList
	config.NodeList = backupRoutine.NodeList
	config.SetList = backupRoutine.SetList

	config.NoRecords = util.ValueOrZero(backupPolicy.NoRecords)
	config.NoIndexes = util.ValueOrZero(backupPolicy.NoIndexes)
	config.NoUDFs = util.ValueOrZero(backupPolicy.NoUdfs)

	config.ParallelRead = backupPolicy.GetParallelOrDefault()
	config.ParallelWrite = backupPolicy.GetParallelOrDefault()
	config.FileLimit = int64(backupPolicy.GetFileLimitOrDefault()) * 1_048_576 // lib expects limit in bytes.
	config.RecordsPerSecond = util.ValueOrZero(backupPolicy.RecordsPerSecond)
	config.Bandwidth = util.ValueOrZero(backupPolicy.Bandwidth) * 1_048_576 // lib expects file size in bytes.

	config.ModBefore = timebounds.ToTime
	config.ModAfter = timebounds.FromTime

	config.ScanPolicy = a.NewScanPolicy()
	config.ScanPolicy.TotalTimeout = backupPolicy.GetTotalTimeoutOrDefault()
	config.ScanPolicy.SocketTimeout = backupPolicy.GetSocketTimeoutOrDefault()

	if backupPolicy.CompressionPolicy != nil {
		config.CompressionPolicy = &backup.CompressionPolicy{
			Mode:  backupPolicy.CompressionPolicy.Mode,
			Level: int(backupPolicy.CompressionPolicy.Level),
		}
	}

	if backupPolicy.EncryptionPolicy != nil {
		config.EncryptionPolicy = &backup.EncryptionPolicy{
			Mode:      backupPolicy.EncryptionPolicy.Mode,
			KeyFile:   backupPolicy.EncryptionPolicy.KeyFile,
			KeySecret: backupPolicy.EncryptionPolicy.KeySecret,
			KeyEnv:    backupPolicy.EncryptionPolicy.KeyEnv,
		}
	}

	config.SecretAgentConfig = secretAgent.ToSecretAgentConfig()

	return config
}
