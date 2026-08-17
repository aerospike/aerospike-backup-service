package backupexecutor

import (
	"context"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/aerospike/backup-go"
)

// runScanBackup performs a regular scan-based backup.
func runScanBackup(
	ctx context.Context,
	client aerospike.Client,
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
	routine *model.BackupRoutine,
	timeBounds model.TimeBounds,
) (*backup.ConfigBackup, error) {
	config := backup.NewDefaultBackupConfig()

	config.Namespace = namespace
	config.BinList = routine.BinList
	config.NodeList = routine.NodeList
	config.SetList = routine.SetList

	if routine.PartitionList != "" {
		// namespace parameter is only applicable for partition by digest; it's not supported by service.
		partitionFilters, err := backup.ParsePartitionFilterListString("", routine.PartitionList)
		if err != nil {
			return nil, fmt.Errorf("failed to parse partition list: %w", err)
		}

		config.PartitionFilters = partitionFilters
	}

	backupPolicy := routine.BackupPolicy
	config.NoRecords = ptr.ValueOrZero(backupPolicy.NoRecords)
	if timeBounds.IsFullBackup() {
		config.NoIndexes = ptr.ValueOrZero(backupPolicy.NoIndexes)
		config.NoUDFs = ptr.ValueOrZero(backupPolicy.NoUdfs)
	} else { // incremental backup don't include indexes or UDFs
		config.NoIndexes = true
		config.NoUDFs = true
	}

	config.ParallelRead = backupPolicy.GetParallelOrDefault()
	config.ParallelWrite = backupPolicy.GetParallelWriteOrDefault()
	config.FileLimit = uint64(backupPolicy.GetFileLimitOrDefault() * megabyte) // lib expects limit in bytes.
	config.RecordsPerSecond = ptr.ValueOrZero(backupPolicy.RecordsPerSecond)
	config.Bandwidth = ptr.ValueOrZero(backupPolicy.Bandwidth) * megabyte // lib expects file size in bytes.
	scanPolicy, err := buildScanPolicy(backupPolicy, routine)
	if err != nil {
		return nil, fmt.Errorf("failed to build scan policy: %w", err)
	}

	config.ScanPolicy = scanPolicy
	config.RackList = routine.RackList // backup only these racks

	config.ModBefore = timeBounds.ToTime
	config.ModAfter = timeBounds.FromTime

	config.CompressionPolicy = makeCompressionPolicy(backupPolicy)
	config.EncryptionPolicy = makeEncryptionPolicy(backupPolicy)
	config.Compact = backupPolicy.CompactOrDefault()
	config.SecretAgentConfig = routine.SecretAgent.ToSecretAgentConfig()

	config.MetricsEnabled = true

	return config, nil
}

func buildScanPolicy(
	backupPolicy *model.BackupPolicy,
	routine *model.BackupRoutine,
) (*as.ScanPolicy, error) {
	scanPolicy := as.NewScanPolicy()
	if backupPolicy.TotalTimeout != nil {
		scanPolicy.TotalTimeout = *backupPolicy.TotalTimeout
	}

	scanPolicy.SocketTimeout = backupPolicy.GetSocketTimeoutOrDefault()
	scanPolicy.UseCompression = backupPolicy.UseCompressionOrDefault()
	scanPolicy.MaxConcurrentNodes = ptr.ValueOrZero(backupPolicy.MaxConcurrentNodes)

	scanPolicy.MaxRetries = int(model.ScanRetryPolicy.MaxRetries)
	scanPolicy.SleepBetweenRetries = model.ScanRetryPolicy.BaseTimeout
	scanPolicy.SleepMultiplier = model.ScanRetryPolicy.Multiplier

	if len(routine.SourceCluster.PreferRacks) > 0 {
		scanPolicy.ReplicaPolicy = as.PREFER_RACK
	}

	if len(routine.RackList) > 0 || len(routine.NodeList) > 0 {
		scanPolicy.ReplicaPolicy = as.MASTER
	}

	if routine.FilterExpression != "" {
		exp, err := as.ExpFromBase64(routine.FilterExpression)
		if err != nil {
			return nil, fmt.Errorf("failed to parse filter expression: %w", err)
		}

		scanPolicy.FilterExpression = exp
	}

	return scanPolicy, nil
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
		KeyFile:   ptr.StringOrNil(policy.EncryptionPolicy.KeyFile),
		KeySecret: ptr.StringOrNil(policy.EncryptionPolicy.KeySecret),
		KeyEnv:    ptr.StringOrNil(policy.EncryptionPolicy.KeyEnv),
	}
}
