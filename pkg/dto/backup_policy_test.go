package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestBackupPolicyConversionIsLossless(t *testing.T) {
	parallel := 4
	socketTimeout := int32(5000)
	totalTimeout := int32(10000)
	retryPolicy := &RetryPolicy{MaxRetries: 3}
	noRecords := true
	noIndexes := false
	noUdfs := true
	bandwidth := 50
	recordsPerSecond := 100
	fileLimit := 1024
	compressionPolicy := &CompressionPolicy{Level: 5}
	sealed := true

	original := &BackupPolicy{
		Parallel:      &parallel,
		SocketTimeout: &socketTimeout,
		TotalTimeout:  &totalTimeout,
		RetryPolicy:   retryPolicy,
		Retention: &RetentionPolicy{
			FullBackups: util.Ptr(10),
			IncrBackups: util.Ptr(5),
		},
		NoRecords:         &noRecords,
		NoIndexes:         &noIndexes,
		NoUdfs:            &noUdfs,
		Bandwidth:         &bandwidth,
		RecordsPerSecond:  &recordsPerSecond,
		FileLimit:         &fileLimit,
		EncryptionPolicy:  nil,
		CompressionPolicy: compressionPolicy,
		Sealed:            &sealed,
	}

	model := original.ToModel()
	result := NewBackupPolicyFromModel(model)

	require.Equal(t, original, result)
}
