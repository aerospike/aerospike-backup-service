package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBackupPolicyConversionIsLossless(t *testing.T) {
	parallel := 4
	socketTimeout := 5000
	totalTimeout := 10000
	retryPolicy := &RetryPolicy{MaxRetries: 3}
	removeFiles := KeepAll
	noRecords := true
	noIndexes := false
	noUdfs := true
	bandwidth := 50
	recordsPerSecond := 100
	fileLimit := 1024
	compressionPolicy := &CompressionPolicy{Level: 5}
	sealed := true

	original := &BackupPolicy{
		Parallel:          &parallel,
		SocketTimeout:     &socketTimeout,
		TotalTimeout:      &totalTimeout,
		RetryPolicy:       retryPolicy,
		RemoveFiles:       &removeFiles,
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
