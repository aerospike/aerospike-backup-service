package backupexecutor

import (
	"context"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/aerospike/backup-go"
)

// runScanBackup performs a regular scan-based backup.
func runScanBackup(
	ctx context.Context,
	client *backup.Client,
	routine *model.BackupRoutine,
	timeBounds model.TimeBounds,
	namespace string,
	writer backup.Writer,
) (BackupHandler, error) {
	config := makeBackupConfig(namespace, routine, timeBounds)

	handler, err := client.Backup(ctx, config, writer, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start scan backup: %w", err)
	}

	return handler, nil
}

const megabyte = 1_048_576

func makeBackupConfig(
	namespace string,
	backupRoutine *model.BackupRoutine,
	timeBounds model.TimeBounds,
) *backup.ConfigBackup {
	config := backup.NewDefaultBackupConfig()

	config.Namespace = namespace
	config.BinList = backupRoutine.BinList
	config.NodeList = backupRoutine.NodeList
	config.SetList = backupRoutine.SetList

	backupPolicy := backupRoutine.BackupPolicy
	config.NoRecords = util.ValueOrZero(backupPolicy.NoRecords)
	if isFullBackup(timeBounds) {
		config.NoIndexes = util.ValueOrZero(backupPolicy.NoIndexes)
		config.NoUDFs = util.ValueOrZero(backupPolicy.NoUdfs)
	} else { // incremental backup don't include indexes or UDFs
		config.NoIndexes = true
		config.NoUDFs = true
	}

	config.ParallelRead = backupPolicy.GetParallelOrDefault()
	config.ParallelWrite = backupPolicy.GetParallelOrDefault()
	config.FileLimit = backupPolicy.GetFileLimitOrDefault() * megabyte // lib expects limit in bytes.
	config.RecordsPerSecond = util.ValueOrZero(backupPolicy.RecordsPerSecond)
	config.Bandwidth = util.ValueOrZero(backupPolicy.Bandwidth) * megabyte // lib expects file size in bytes.

	config.ModBefore = timeBounds.ToTime
	config.ModAfter = timeBounds.FromTime

	config.ScanPolicy = as.NewScanPolicy()
	if backupPolicy.TotalTimeout != nil {
		config.ScanPolicy.TotalTimeout = *backupPolicy.TotalTimeout
	}
	if backupPolicy.SocketTimeout != nil {
		config.ScanPolicy.SocketTimeout = *backupPolicy.SocketTimeout
	}
	config.ScanPolicy.MaxRetries = 100

	config.CompressionPolicy = makeCompressionPolicy(backupPolicy)
	config.EncryptionPolicy = makeEncryptionPolicy(backupPolicy)

	return config
}

func makeCompressionPolicy(policy *model.BackupPolicy) *backup.CompressionPolicy {
	if policy == nil || policy.CompressionPolicy == nil {
		return nil
	}

	return &backup.CompressionPolicy{
		Mode:  policy.CompressionPolicy.Mode,
		Level: int(policy.CompressionPolicy.Level),
	}
}

func makeEncryptionPolicy(policy *model.BackupPolicy) *backup.EncryptionPolicy {
	if policy == nil || policy.EncryptionPolicy == nil {
		return nil
	}

	return &backup.EncryptionPolicy{
		Mode:      policy.EncryptionPolicy.Mode,
		KeyFile:   policy.EncryptionPolicy.KeyFile,
		KeySecret: policy.EncryptionPolicy.KeySecret,
		KeyEnv:    policy.EncryptionPolicy.KeyEnv,
	}
}
