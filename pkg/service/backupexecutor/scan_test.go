package backupexecutor

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMakeBackupConfigWithFullBackup(t *testing.T) {
	namespace := "testNamespace"
	routine := &model.BackupRoutine{
		BackupPolicy: &model.BackupPolicy{
			Parallel:      ptr.Of(4),
			ParallelWrite: ptr.Of(8),
			FileLimit:     ptr.Of(100),
			NoRecords:     ptr.Of(true),
			NoIndexes:     ptr.Of(true),
			NoUdfs:        ptr.Of(true),
			Bandwidth:     ptr.Of(int64(10)),
			SocketTimeout: ptr.Of(5 * time.Minute),

			CompressionPolicy: &model.CompressionPolicy{
				Mode:  "ZSTD",
				Level: 3,
			},
			EncryptionPolicy: &model.EncryptionPolicy{
				Mode:      "aes-256",
				KeyFile:   ptr.Of("path/to/key"),
				KeySecret: ptr.Of("secret-name"),
				KeyEnv:    ptr.Of("ENV_VAR"),
			},
			Compact: ptr.Of(true),
		},

		SetList:      []string{"testSet"},
		BinList:      []string{"bin1", "bin2"},
		NodeList:     []string{"node1", "node2"},
		IntervalCron: "@daily",
		SecretAgent: &model.SecretAgent{
			ConnectionType: "tcp",
			Address:        "localhost",
			Port:           ptr.Of(model.Port(9000)),
			Timeout:        ptr.Of(1000),
			ClientTLS:      model.ClientTLS{CAFile: ptr.Of("ca-string")},
			IsBase64:       ptr.Of(true),
		},
		SourceCluster: &model.AerospikeCluster{},
	}

	// Full backup has FromTime == nil
	timeBounds := model.TimeBounds{
		FromTime: nil,
		ToTime:   ptr.Of(time.Now()),
	}

	config, err := makeBackupConfig(namespace, routine, timeBounds)

	require.NoError(t, err)
	assert.Equal(t, namespace, config.Namespace)
	assert.Equal(t, routine.BinList, config.BinList)
	assert.Equal(t, routine.SetList, config.SetList)
	assert.Equal(t, routine.NodeList, config.NodeList)
	assert.Equal(t, 4, config.ParallelRead)
	assert.Equal(t, 8, config.ParallelWrite)
	assert.Equal(t, uint64(100*megabyte), config.FileLimit)
	assert.True(t, config.NoRecords)
	assert.True(t, config.NoIndexes)
	assert.True(t, config.NoUDFs)
	assert.Equal(t, int64(10*megabyte), config.Bandwidth)
	assert.Equal(t, timeBounds.ToTime, config.ModBefore)
	assert.Nil(t, config.ModAfter)
	assert.NotNil(t, config.ScanPolicy)
	assert.Equal(t, 10, config.ScanPolicy.MaxRetries)
	assert.True(t, config.MetricsEnabled)

	assert.NotNil(t, config.SecretAgentConfig)
	assert.Equal(t, "localhost", *config.SecretAgentConfig.Address)
	assert.Equal(t, 9000, *config.SecretAgentConfig.Port)
	assert.Equal(t, "tcp", *config.SecretAgentConfig.ConnectionType)
	assert.Equal(t, 1000, *config.SecretAgentConfig.TimeoutMillisecond)
	assert.Equal(t, "ca-string", *config.SecretAgentConfig.CaFile)
	assert.True(t, *config.SecretAgentConfig.IsBase64)

	encryptionPolicy := config.EncryptionPolicy
	require.NotNil(t, encryptionPolicy)
	assert.Equal(t, "aes-256", encryptionPolicy.Mode)
	assert.Equal(t, "path/to/key", *encryptionPolicy.KeyFile)
	assert.Equal(t, "secret-name", *encryptionPolicy.KeySecret)
	assert.Equal(t, "ENV_VAR", *encryptionPolicy.KeyEnv)

	compressionPolicy := config.CompressionPolicy
	require.NotNil(t, compressionPolicy)
	assert.Equal(t, "ZSTD", compressionPolicy.Mode)
	assert.Equal(t, 3, compressionPolicy.Level)
	assert.True(t, config.Compact)
}

func TestMakeBackupConfigWithIncrementalBackup(t *testing.T) {
	namespace := "testNamespace"
	routine := &model.BackupRoutine{
		BackupPolicy: &model.BackupPolicy{
			Parallel:      ptr.Of(4),
			ParallelWrite: ptr.Of(8),
			FileLimit:     ptr.Of(100),
			NoRecords:     ptr.Of(false),
			NoIndexes:     ptr.Of(false),
			NoUdfs:        ptr.Of(false),
			Bandwidth:     ptr.Of(int64(10)),
		},
		SetList:          []string{"testSet"},
		BinList:          []string{"bin1", "bin2"},
		NodeList:         []string{"node1", "node2"},
		IncrIntervalCron: "@hourly",
		SecretAgent:      &model.SecretAgent{},
		SourceCluster:    &model.AerospikeCluster{},
	}

	// Incremental backup has FromTime != nil
	fromTime := time.Now().Add(-1 * time.Hour)
	timeBounds := model.TimeBounds{
		FromTime: &fromTime,
		ToTime:   ptr.Of(time.Now()),
	}

	config, err := makeBackupConfig(namespace, routine, timeBounds)

	require.NoError(t, err)
	assert.Equal(t, namespace, config.Namespace)
	assert.Equal(t, routine.BinList, config.BinList)
	assert.Equal(t, routine.SetList, config.SetList)
	assert.Equal(t, routine.NodeList, config.NodeList)
	assert.Equal(t, 4, config.ParallelRead)
	assert.Equal(t, 8, config.ParallelWrite)
	assert.Equal(t, uint64(100*megabyte), config.FileLimit)
	assert.False(t, config.NoRecords)
	assert.True(t, config.NoIndexes) // Incremental backup should have NoIndexes=true
	assert.True(t, config.NoUDFs)    // Incremental backup should have NoUDFs=true
	assert.Equal(t, int64(10*megabyte), config.Bandwidth)
	assert.Equal(t, timeBounds.ToTime, config.ModBefore)
	assert.Equal(t, timeBounds.FromTime, config.ModAfter)
	assert.NotNil(t, config.ScanPolicy)
}

func TestMakeBackupConfigWithPartitionList(t *testing.T) {
	namespace := "testNamespace"
	routine := &model.BackupRoutine{
		BackupPolicy:  &model.BackupPolicy{},
		PartitionList: "0-100",
		IntervalCron:  "@daily",
		SourceCluster: &model.AerospikeCluster{},
	}

	config, err := makeBackupConfig(namespace, routine, model.TimeBounds{})

	require.NoError(t, err)
	assert.NotNil(t, config.PartitionFilters)
	assert.Len(t, config.PartitionFilters, 1)

	assert.Nil(t, config.EncryptionPolicy)
	assert.Nil(t, config.CompressionPolicy)
}

func TestMakeBackupConfig_DefaultParallelWrite(t *testing.T) {
	namespace := "testNamespace"
	routine := &model.BackupRoutine{
		BackupPolicy: &model.BackupPolicy{
			Parallel: ptr.Of(4),
		},
		IntervalCron:  "@daily",
		SourceCluster: &model.AerospikeCluster{},
	}

	config, err := makeBackupConfig(namespace, routine, model.TimeBounds{})

	require.NoError(t, err)
	assert.Equal(t, 4, config.ParallelRead)
	assert.Equal(t, 4, config.ParallelWrite)
}

func TestMakeBackupConfigWithPreferRacks(t *testing.T) {
	namespace := "testNamespace"
	routine := &model.BackupRoutine{
		BackupPolicy: &model.BackupPolicy{},
		IntervalCron: "@daily",
		SourceCluster: &model.AerospikeCluster{
			PreferRacks: []int{1, 2, 3},
		},
	}

	config, err := makeBackupConfig(namespace, routine, model.TimeBounds{})

	require.NoError(t, err)
	assert.NotNil(t, config.ScanPolicy)
	assert.Equal(t, as.PREFER_RACK, config.ScanPolicy.ReplicaPolicy)
	assert.Nil(t, config.EncryptionPolicy)
	assert.Nil(t, config.CompressionPolicy)
}

func TestMakeBackupConfigWithRackList(t *testing.T) {
	namespace := "testNamespace"
	routine := &model.BackupRoutine{
		RackList:      []int{1, 2, 3},
		BackupPolicy:  &model.BackupPolicy{},
		IntervalCron:  "@daily",
		SourceCluster: &model.AerospikeCluster{},
	}

	config, err := makeBackupConfig(namespace, routine, model.TimeBounds{})

	require.NoError(t, err)
	assert.NotNil(t, config.ScanPolicy)
	assert.Equal(t, as.MASTER, config.ScanPolicy.ReplicaPolicy)
	assert.Nil(t, config.EncryptionPolicy)
	assert.Nil(t, config.CompressionPolicy)
}

func TestMakeBackupConfigWithNodeList(t *testing.T) {
	namespace := "testNamespace"
	routine := &model.BackupRoutine{
		NodeList:      []string{"node1", "node2", "node3"},
		BackupPolicy:  &model.BackupPolicy{},
		IntervalCron:  "@daily",
		SourceCluster: &model.AerospikeCluster{},
	}

	config, err := makeBackupConfig(namespace, routine, model.TimeBounds{})

	require.NoError(t, err)
	assert.NotNil(t, config.ScanPolicy)
	assert.Equal(t, as.MASTER, config.ScanPolicy.ReplicaPolicy)
	assert.Nil(t, config.EncryptionPolicy)
	assert.Nil(t, config.CompressionPolicy)
}

func TestMakeBackupConfigWithFilterExpression(t *testing.T) {
	namespace := "testNamespace"
	routine := &model.BackupRoutine{
		BackupPolicy:     &model.BackupPolicy{},
		IntervalCron:     "@daily",
		SourceCluster:    &model.AerospikeCluster{},
		FilterExpression: "k1EDpHRlc3Q=",
	}

	config, err := makeBackupConfig(namespace, routine, model.TimeBounds{})

	require.NoError(t, err)
	require.NotNil(t, config.ScanPolicy)
	require.NotNil(t, config.ScanPolicy.FilterExpression)
}

func TestMakeBackupConfigWithInvalidFilterExpression(t *testing.T) {
	namespace := "testNamespace"
	routine := &model.BackupRoutine{
		BackupPolicy:     &model.BackupPolicy{},
		IntervalCron:     "@daily",
		SourceCluster:    &model.AerospikeCluster{},
		FilterExpression: "invalid-exp",
	}

	_, err := makeBackupConfig(namespace, routine, model.TimeBounds{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse filter expression")
}
