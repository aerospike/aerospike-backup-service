package restoreexecutor

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMakeRestoreConfigWithFullParams(t *testing.T) {
	restoreRequest := &model.RestoreRequest{
		Policy: &model.RestorePolicy{
			BinList:            []string{"bin1", "bin2"},
			SetList:            []string{"set1", "set2"},
			NoRecords:          util.Ptr(false),
			NoIndexes:          util.Ptr(false),
			NoUdfs:             util.Ptr(false),
			Tps:                util.Ptr(500),
			Bandwidth:          util.Ptr(1024),
			ExtraTTL:           util.Ptr(int64(3600)),
			Parallel:           util.Ptr(8),
			BatchSize:          util.Ptr(128),
			MaxAsyncBatches:    util.Ptr(10),
			DisableBatchWrites: util.Ptr(true),

			Namespace: &model.RestoreNamespace{
				Source:      util.Ptr("ns_source"),
				Destination: util.Ptr("ns_dest"),
			},

			CompressionPolicy: &model.CompressionPolicy{
				Mode:  "ZSTD",
				Level: 4,
			},

			EncryptionPolicy: &model.EncryptionPolicy{
				Mode:      "aes-128",
				KeyFile:   util.Ptr("key/file/path"),
				KeySecret: util.Ptr("key-secret"),
				KeyEnv:    util.Ptr("KEY_ENV_VAR"),
			},
		},
		SecretAgent: &model.SecretAgent{
			ConnectionType: "tcp",
			Address:        "127.0.0.1",
			Port:           util.Ptr(model.Port(1234)),
			Timeout:        util.Ptr(2000),
			TLSCAString:    util.Ptr("ca-cert"),
			IsBase64:       util.Ptr(true),
		},
	}

	config := makeRestoreConfig(restoreRequest)

	require.NotNil(t, config)
	assert.Equal(t, []string{"bin1", "bin2"}, config.BinList)
	assert.Equal(t, []string{"set1", "set2"}, config.SetList)
	assert.Equal(t, false, config.NoRecords)
	assert.Equal(t, false, config.NoIndexes)
	assert.Equal(t, false, config.NoUDFs)
	assert.Equal(t, 500, config.RecordsPerSecond)
	assert.Equal(t, 1024, config.Bandwidth)
	assert.Equal(t, int64(3600), config.ExtraTTL)
	assert.Equal(t, 8, config.Parallel)
	assert.Equal(t, 128, config.BatchSize)
	assert.Equal(t, 10, config.MaxAsyncBatches)
	assert.Equal(t, true, config.DisableBatchWrites)
	assert.Equal(t, config.MetricsEnabled, true)

	require.NotNil(t, config.Namespace)
	assert.Equal(t, "ns_source", *config.Namespace.Source)
	assert.Equal(t, "ns_dest", *config.Namespace.Destination)

	require.NotNil(t, config.CompressionPolicy)
	assert.Equal(t, "ZSTD", config.CompressionPolicy.Mode)
	assert.Equal(t, 4, config.CompressionPolicy.Level)

	require.NotNil(t, config.EncryptionPolicy)
	assert.Equal(t, "aes-128", config.EncryptionPolicy.Mode)
	assert.Equal(t, "key/file/path", *config.EncryptionPolicy.KeyFile)
	assert.Equal(t, "key-secret", *config.EncryptionPolicy.KeySecret)
	assert.Equal(t, "KEY_ENV_VAR", *config.EncryptionPolicy.KeyEnv)

	require.NotNil(t, config.SecretAgentConfig)
	assert.Equal(t, "127.0.0.1", *config.SecretAgentConfig.Address)
	assert.Equal(t, 1234, *config.SecretAgentConfig.Port)
	assert.Equal(t, "tcp", *config.SecretAgentConfig.ConnectionType)
	assert.Equal(t, 2000, *config.SecretAgentConfig.TimeoutMillisecond)
	assert.Equal(t, "ca-cert", *config.SecretAgentConfig.CaFile)
	assert.Equal(t, true, *config.SecretAgentConfig.IsBase64)
}
