package service

import (
	"context"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	a "github.com/aerospike/aerospike-client-go/v8"
	"github.com/aerospike/backup-go"
)

// BackupExecutor implements the [Backup] interface.
type BackupExecutor struct {
	backupRoutine *model.BackupRoutine
}

func NewBackupExecutor(
	backupRoutine *model.BackupRoutine,
) *BackupExecutor {
	return &BackupExecutor{
		backupRoutine: backupRoutine,
	}
}

// BackupRun creates a [backup.Client] and initiates the backup operation.
// A backup handler is returned to monitor the job status.
func (b *BackupExecutor) BackupRun(
	ctx context.Context,
	client *backup.Client,
	policy *model.BackupPolicy,
	timeBounds model.TimeBounds,
	namespace string,
	path string,
) (BackupHandler, error) {
	config := makeBackupConfig(namespace, b.backupRoutine, policy, timeBounds, b.backupRoutine.SecretAgent)

	writerFactory, err := storage.CreateWriter(ctx, b.backupRoutine.Storage, path, false, false, false)
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
	timeBounds model.TimeBounds,
	secretAgent *model.SecretAgent,
) *backup.ConfigBackup {
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

	config.ModBefore = timeBounds.ToTime
	config.ModAfter = timeBounds.FromTime

	config.ScanPolicy = a.NewScanPolicy()
	if backupPolicy.TotalTimeout != nil {
		config.ScanPolicy.TotalTimeout = *backupPolicy.TotalTimeout
	}
	if backupPolicy.SocketTimeout != nil {
		config.ScanPolicy.SocketTimeout = *backupPolicy.SocketTimeout
	}
	config.ScanPolicy.MaxRetries = 100

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
