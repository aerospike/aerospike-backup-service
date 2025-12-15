package restoreexecutor

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMakeRestoreConfig(t *testing.T) {
	restoreRequest := &model.RestoreRequest{
		Policy: &model.RestorePolicy{
			BinList:            []string{"bin1", "bin2"},
			SetList:            []string{"set1", "set2"},
			NoRecords:          ptr.Of(false),
			NoIndexes:          ptr.Of(false),
			NoUdfs:             ptr.Of(false),
			Tps:                ptr.Of(500),
			Bandwidth:          ptr.Of(int64(1024)),
			ExtraTTL:           ptr.Of(int64(3600)),
			Parallel:           ptr.Of(8),
			BatchSize:          ptr.Of(128),
			MaxAsyncBatches:    ptr.Of(10),
			DisableBatchWrites: ptr.Of(true),

			Namespace: &model.RestoreNamespace{
				Source:      ptr.Of("ns_source"),
				Destination: ptr.Of("ns_dest"),
			},

			CompressionPolicy: &model.CompressionPolicy{
				Mode:  "ZSTD",
				Level: 4,
			},

			EncryptionPolicy: &model.EncryptionPolicy{
				Mode:      "aes-128",
				KeyFile:   ptr.Of("key/file/path"),
				KeySecret: ptr.Of("key-secret"),
				KeyEnv:    ptr.Of("KEY_ENV_VAR"),
			},
		},
		SecretAgent: &model.SecretAgent{
			ConnectionType: "tcp",
			Address:        "127.0.0.1",
			Port:           ptr.Of(model.Port(1234)),
			Timeout:        ptr.Of(2000),
			ClientTLS:      model.ClientTLS{CAFile: ptr.Of("ca-cert")},
			IsBase64:       ptr.Of(true),
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
	assert.Equal(t, int64(1024*megabyte), config.Bandwidth)
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

func TestRecordExistsAction(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name     string
		replace  *bool
		unique   *bool
		expected as.RecordExistsAction
	}{
		{
			name:     "replace=true, unique=nil",
			replace:  &trueVal,
			unique:   nil,
			expected: as.REPLACE,
		},
		{
			name:     "replace=true, unique=false",
			replace:  &trueVal,
			unique:   &falseVal,
			expected: as.REPLACE,
		},
		{
			name:     "replace=false, unique=true",
			replace:  &falseVal,
			unique:   &trueVal,
			expected: as.CREATE_ONLY,
		},
		{
			name:     "replace=nil, unique=true",
			replace:  nil,
			unique:   &trueVal,
			expected: as.CREATE_ONLY,
		},
		{
			name:     "replace=nil, unique=nil",
			replace:  nil,
			unique:   nil,
			expected: as.UPDATE,
		},
		{
			name:     "replace=false, unique=false",
			replace:  &falseVal,
			unique:   &falseVal,
			expected: as.UPDATE,
		},
		{
			name:     "replace=false, unique=nil",
			replace:  &falseVal,
			unique:   nil,
			expected: as.UPDATE,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := recordExistsAction(tt.replace, tt.unique)
			assert.Equal(t, tt.expected, result)
		})
	}
}
