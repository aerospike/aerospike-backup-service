package backupexecutor

import (
	"context"
	"fmt"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	a "github.com/aerospike/aerospike-client-go/v8"
	"github.com/aerospike/backup-go"
)

// DefaultBackupExecutor implements the actual backup logic.
type DefaultBackupExecutor struct{}

func NewDefaultBackupExecutor() *DefaultBackupExecutor {
	return &DefaultBackupExecutor{}
}

// Run implements the backup logic.
func (r *DefaultBackupExecutor) Run(
	ctx context.Context,
	client *backup.Client,
	routine *model.BackupRoutine,
	timeBounds model.TimeBounds,
	namespace string,
	path string,
) (BackupHandler, error) {
	xdrEnabled := routine.BackupPolicy.XDRConfig != nil
	writer, err := storage.CreateWriter(ctx, routine.Storage, path, false, false, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup writer: %w", err)
	}

	switch {
	case !xdrEnabled:
		// Regular scan backup
		return runScanBackup(ctx, client, routine, timeBounds, namespace, writer)
	case isFullBackup(timeBounds):
		// Full backup with XDR - combine XDR for records and scan for UDFs/indexes
		return runCombinedBackup(ctx, client, routine, timeBounds, namespace, writer)
	default:
		// Incremental backup with XDR
		return runXDRBackup(ctx, client, routine, timeBounds, namespace, writer)
	}
}

func isFullBackup(timeBounds model.TimeBounds) bool {
	return timeBounds.FromTime == nil
}

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
	config.SecretAgentConfig = backupRoutine.SecretAgent.ToSecretAgentConfig()

	return config
}

// runXDRBackup performs an XDR-based backup.
func runXDRBackup(
	ctx context.Context,
	client *backup.Client,
	routine *model.BackupRoutine,
	timeBounds model.TimeBounds,
	namespace string,
	writer backup.Writer,
) (BackupHandler, error) {
	xdrConfig := makeXDRConfig(namespace, routine, timeBounds)

	handler, err := client.BackupXDR(ctx, xdrConfig, writer)
	if err != nil {
		return nil, fmt.Errorf("failed to start XDR backup: %w", err)
	}

	return handler, nil
}

func makeXDRConfig(
	namespace string, routine *model.BackupRoutine, timeBounds model.TimeBounds,
) *backup.ConfigBackupXDR {
	policy := routine.BackupPolicy
	return &backup.ConfigBackupXDR{
		DC:                           policy.XDRConfig.DC,
		LocalAddress:                 policy.XDRConfig.LocalHost,
		LocalPort:                    policy.XDRConfig.LocalPort,
		Namespace:                    namespace,
		Rewind:                       getRewind(timeBounds),
		ParallelWrite:                policy.GetParallelOrDefault(),
		FileLimit:                    int64(policy.GetFileLimitOrDefault()) * 1_048_576,
		MaxConnections:               100,
		ReadTimeoutMilliseconds:      1000,
		WriteTimeoutMilliseconds:     1000,
		StartTimeoutMilliseconds:     10_000,
		InfoPolingPeriodMilliseconds: 1000,
		SecretAgentConfig:            routine.SecretAgent.ToSecretAgentConfig(),
		EncoderType:                  backup.EncoderTypeASBX,
		//CompressionPolicy:            makeCompressionPolicy(policy),
		//EncryptionPolicy:             makeEncryptionPolicy(policy),
	}
}

// getRewind calculates the rewind value based on FromTime.
func getRewind(bounds model.TimeBounds) string {
	if bounds.FromTime == nil {
		return "all"
	}
	seconds := int(time.Since(*bounds.FromTime).Seconds()) + 1

	return fmt.Sprintf("%d", seconds)
}

// runCombinedBackup performs both XDR backup for records and scan backup for UDFs/indexes.
func runCombinedBackup(
	ctx context.Context,
	client *backup.Client,
	routine *model.BackupRoutine,
	timeBounds model.TimeBounds,
	namespace string,
	writer backup.Writer,
) (BackupHandler, error) {
	xdrHandler, err := runXDRBackup(ctx, client, routine, timeBounds, namespace, writer)
	if err != nil {
		return nil, fmt.Errorf("failed to start XDR backup: %w", err)
	}

	// For scan backup, create a copy of routine with NoRecords set to true.
	scanRoutine := *routine
	scanRoutine.BackupPolicy = routine.BackupPolicy.CopyWithNoRecords()

	scanHandler, err := runScanBackup(ctx, client, &scanRoutine, timeBounds, namespace, writer)
	if err != nil {
		return nil, fmt.Errorf("failed to start scan backup: %w", err)
	}

	return &CombinedBackupHandler{
		xdrHandler:  xdrHandler,
		scanHandler: scanHandler,
	}, nil
}
