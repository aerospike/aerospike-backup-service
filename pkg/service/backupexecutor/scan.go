package backupexecutor

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/aerospike/backup-go"
	"github.com/reugn/go-quartz/quartz"
)

// runScanBackup performs a regular scan-based backup.
func runScanBackup(
	ctx context.Context,
	client aerospike.Backuper,
	routine *model.BackupRoutine,
	timeBounds model.TimeBounds,
	namespace string,
	writer backup.Writer,
) (BackupHandler, error) {
	config, err := makeBackupConfig(namespace, routine, timeBounds)
	if err != nil {
		return nil, fmt.Errorf("failed to make backup config: %w", err)
	}

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
) (*backup.ConfigBackup, error) {
	config := backup.NewDefaultBackupConfig()

	config.Namespace = namespace
	config.BinList = backupRoutine.BinList
	config.NodeList = backupRoutine.NodeList
	config.SetList = backupRoutine.SetList

	if backupRoutine.PartitionList != "" {
		// namespace parameter is only applicable for partition by digest; it's not supported by service.
		partitionFilters, err := backup.ParsePartitionFilterListString("", backupRoutine.PartitionList)
		if err != nil {
			return nil, fmt.Errorf("failed to parse partition list: %w", err)
		}

		config.PartitionFilters = partitionFilters
	}

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
	config.ParallelWrite = backupPolicy.GetParallelWriteOrDefault()
	config.FileLimit = uint64(backupPolicy.GetFileLimitOrDefault() * megabyte) // lib expects limit in bytes.
	config.RecordsPerSecond = util.ValueOrZero(backupPolicy.RecordsPerSecond)
	config.Bandwidth = util.ValueOrZero(backupPolicy.Bandwidth) * megabyte // lib expects file size in bytes.

	config.ModBefore = timeBounds.ToTime
	config.ModAfter = timeBounds.FromTime

	config.ScanPolicy = as.NewScanPolicy()
	if backupPolicy.TotalTimeout != nil {
		config.ScanPolicy.TotalTimeout = *backupPolicy.TotalTimeout
	}

	config.ScanPolicy.SocketTimeout = calculateSocketTimeout(backupRoutine, isFullBackup(timeBounds), time.Now())
	config.ScanPolicy.UseCompression = util.ValueOrZero(backupPolicy.UseCompression)

	config.ScanPolicy.MaxRetries = 10
	config.ScanPolicy.SleepBetweenRetries = 1 * time.Second
	config.ScanPolicy.SleepMultiplier = 2

	config.CompressionPolicy = makeCompressionPolicy(backupPolicy)
	config.EncryptionPolicy = makeEncryptionPolicy(backupPolicy)
	config.SecretAgentConfig = backupRoutine.SecretAgent.ToSecretAgentConfig()

	config.MetricsEnabled = true

	config.RackList = backupRoutine.PreferRacks

	return config, nil
}

// calculateSocketTimeout calculates socket timeout for the given backup routine and timestamp.
// timeout should not exceed the next interval trigger.
func calculateSocketTimeout(routine *model.BackupRoutine, isFullBackup bool, now time.Time) time.Duration {
	var timeout = model.DefaultSocketTimeout
	if routine.BackupPolicy.SocketTimeout != nil && *routine.BackupPolicy.SocketTimeout != 0 {
		timeout = *routine.BackupPolicy.SocketTimeout
	}

	// If timeout is 0, treat as infinite
	if timeout == 0 {
		timeout = time.Duration(math.MaxInt64)
	}

	nextTrigger := timeToNextTrigger(routine, isFullBackup, now)

	return min(timeout, nextTrigger, model.DefaultSocketTimeout)
}

func timeToNextTrigger(routine *model.BackupRoutine, isFullBackup bool, now time.Time) time.Duration {
	var cron string
	if isFullBackup {
		cron = routine.IntervalCron
	} else {
		cron = routine.IncrIntervalCron
	}

	cronTrigger, _ := quartz.NewCronTrigger(cron)
	fireTime, _ := cronTrigger.NextFireTime(now.UnixNano())
	delta := time.Unix(0, fireTime).Sub(now)

	return delta
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
