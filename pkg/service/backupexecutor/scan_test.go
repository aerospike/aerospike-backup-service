package backupexecutor

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var now = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

func TestCalculateSocketTimeout(t *testing.T) {
	tests := []struct {
		name         string
		routine      *model.BackupRoutine
		isFullBackup bool
		expected     time.Duration
	}{
		{
			name: "explicit timeout less than other constraints",
			routine: &model.BackupRoutine{
				IntervalCron: "@daily",
				BackupPolicy: &model.BackupPolicy{
					SocketTimeout: util.Ptr(5 * time.Minute),
				},
			},
			isFullBackup: true,
			expected:     5 * time.Minute,
		},
		{
			name: "next backup soon",
			routine: &model.BackupRoutine{
				IntervalCron: "0 */1 * * * *", // every minute
				BackupPolicy: &model.BackupPolicy{
					SocketTimeout: util.Ptr(2 * time.Minute),
				},
			},
			isFullBackup: true,
			expected:     time.Minute,
		},
		{
			name: "default cap of 10 minutes applies",
			routine: &model.BackupRoutine{
				IntervalCron: "@daily",
				BackupPolicy: &model.BackupPolicy{
					SocketTimeout: nil, // No explicit timeout
				},
			},
			isFullBackup: true,
			expected:     model.DefaultSocketTimeout,
		},
		{
			name: "user set 0",
			routine: &model.BackupRoutine{
				IntervalCron: "@daily",
				BackupPolicy: &model.BackupPolicy{
					SocketTimeout: util.Ptr(0 * time.Second),
				},
			},
			isFullBackup: true,
			expected:     model.DefaultSocketTimeout,
		},
		{
			name: "incremental backup",
			routine: &model.BackupRoutine{
				IncrIntervalCron: "@hourly",
				BackupPolicy: &model.BackupPolicy{
					SocketTimeout: util.Ptr(3 * time.Minute),
				},
			},
			isFullBackup: false,
			expected:     3 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateSocketTimeout(tt.routine, tt.isFullBackup, now)
			assert.Equal(t, result, tt.expected)
		})
	}
}

func TestMakeBackupConfigWithFullBackup(t *testing.T) {
	namespace := "testNamespace"
	routine := &model.BackupRoutine{
		BackupPolicy: &model.BackupPolicy{
			Parallel:      util.Ptr(4),
			FileLimit:     util.Ptr(100),
			NoRecords:     util.Ptr(false),
			NoIndexes:     util.Ptr(false),
			NoUdfs:        util.Ptr(false),
			Bandwidth:     util.Ptr(10),
			SocketTimeout: util.Ptr(5 * time.Minute),

			CompressionPolicy: &model.CompressionPolicy{
				Mode:  "ZSTD",
				Level: 3,
			},
			EncryptionPolicy: &model.EncryptionPolicy{
				Mode:      "aes-256",
				KeyFile:   util.Ptr("path/to/key"),
				KeySecret: util.Ptr("secret-name"),
				KeyEnv:    util.Ptr("ENV_VAR"),
			},
		},

		SetList:      []string{"testSet"},
		BinList:      []string{"bin1", "bin2"},
		NodeList:     []string{"node1", "node2"},
		IntervalCron: "@daily",
		SecretAgent: &model.SecretAgent{
			ConnectionType: "tcp",
			Address:        "localhost",
			Port:           util.Ptr(model.Port(9000)),
			Timeout:        util.Ptr(1000),
			TLSCAString:    util.Ptr("ca-string"),
			IsBase64:       util.Ptr(true),
		},
	}

	// Full backup has FromTime == nil
	timeBounds := model.TimeBounds{
		FromTime: nil,
		ToTime:   util.Ptr(time.Now()),
	}

	config, err := makeBackupConfig(namespace, routine, timeBounds)

	require.NoError(t, err)
	assert.Equal(t, namespace, config.Namespace)
	assert.Equal(t, routine.BinList, config.BinList)
	assert.Equal(t, routine.SetList, config.SetList)
	assert.Equal(t, routine.NodeList, config.NodeList)
	assert.Equal(t, 4, config.ParallelRead)
	assert.Equal(t, 4, config.ParallelWrite)
	assert.Equal(t, uint64(100*megabyte), config.FileLimit)
	assert.Equal(t, false, config.NoRecords)
	assert.Equal(t, false, config.NoIndexes)
	assert.Equal(t, false, config.NoUDFs)
	assert.Equal(t, 10*megabyte, config.Bandwidth)
	assert.Equal(t, timeBounds.ToTime, config.ModBefore)
	assert.Nil(t, config.ModAfter)
	assert.NotNil(t, config.ScanPolicy)
	assert.Equal(t, 100, config.ScanPolicy.MaxRetries)
	assert.Equal(t, config.MetricsEnabled, true)

	assert.NotNil(t, config.SecretAgentConfig)
	assert.Equal(t, *config.SecretAgentConfig.Address, "localhost")
	assert.Equal(t, *config.SecretAgentConfig.Port, 9000)
	assert.Equal(t, *config.SecretAgentConfig.ConnectionType, "tcp")
	assert.Equal(t, *config.SecretAgentConfig.TimeoutMillisecond, 1000)
	assert.Equal(t, *config.SecretAgentConfig.CaFile, "ca-string")
	assert.Equal(t, *config.SecretAgentConfig.IsBase64, true)

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
}

func TestMakeBackupConfigWithIncrementalBackup(t *testing.T) {
	namespace := "testNamespace"
	routine := &model.BackupRoutine{
		BackupPolicy: &model.BackupPolicy{
			Parallel:  util.Ptr(4),
			FileLimit: util.Ptr(100),
			NoRecords: util.Ptr(false),
			NoIndexes: util.Ptr(false),
			NoUdfs:    util.Ptr(false),
			Bandwidth: util.Ptr(10),
		},
		SetList:          []string{"testSet"},
		BinList:          []string{"bin1", "bin2"},
		NodeList:         []string{"node1", "node2"},
		IncrIntervalCron: "@hourly",
		SecretAgent:      &model.SecretAgent{},
	}

	// Incremental backup has FromTime != nil
	fromTime := time.Now().Add(-1 * time.Hour)
	timeBounds := model.TimeBounds{
		FromTime: &fromTime,
		ToTime:   util.Ptr(time.Now()),
	}

	config, err := makeBackupConfig(namespace, routine, timeBounds)

	require.NoError(t, err)
	assert.Equal(t, namespace, config.Namespace)
	assert.Equal(t, routine.BinList, config.BinList)
	assert.Equal(t, routine.SetList, config.SetList)
	assert.Equal(t, routine.NodeList, config.NodeList)
	assert.Equal(t, 4, config.ParallelRead)
	assert.Equal(t, 4, config.ParallelWrite)
	assert.Equal(t, uint64(100*megabyte), config.FileLimit)
	assert.Equal(t, false, config.NoRecords)
	assert.Equal(t, true, config.NoIndexes) // Incremental backup should have NoIndexes=true
	assert.Equal(t, true, config.NoUDFs)    // Incremental backup should have NoUDFs=true
	assert.Equal(t, 10*megabyte, config.Bandwidth)
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
	}

	config, err := makeBackupConfig(namespace, routine, model.TimeBounds{})

	require.NoError(t, err)
	assert.NotNil(t, config.PartitionFilters)
	assert.Len(t, config.PartitionFilters, 1)

	assert.Nil(t, config.EncryptionPolicy)
	assert.Nil(t, config.CompressionPolicy)
}
