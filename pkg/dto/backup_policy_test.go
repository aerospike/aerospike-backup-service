package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestBackupPolicyConversionIsLossless(t *testing.T) {
	parallel := 4
	socketTimeout := 5000
	totalTimeout := 10000
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
		RetentionPolicy: &RetentionPolicy{
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

func TestRetentionPolicy_Validate(t *testing.T) {
	tests := []struct {
		name        string
		policy      *RetentionPolicy
		expectedErr string
	}{
		{
			name:        "nil policy (no validation)",
			policy:      nil,
			expectedErr: "",
		},
		{
			name:        "valid policy with both full and incremental backups",
			policy:      &RetentionPolicy{FullBackups: util.Ptr(5), IncrBackups: util.Ptr(3)},
			expectedErr: "",
		},
		{
			name:        "valid policy with only full backups",
			policy:      &RetentionPolicy{FullBackups: util.Ptr(2), IncrBackups: nil},
			expectedErr: "",
		},
		{
			name:        "valid policy with only incremental backups set to zero",
			policy:      &RetentionPolicy{FullBackups: util.Ptr(3), IncrBackups: util.Ptr(0)},
			expectedErr: "",
		},
		{
			name:        "invalid full backups: less than 1",
			policy:      &RetentionPolicy{FullBackups: util.Ptr(0), IncrBackups: util.Ptr(1)},
			expectedErr: "full backups retention 0 is invalid, must be at least 1",
		},
		{
			name:        "invalid incremental backups: negative value",
			policy:      &RetentionPolicy{FullBackups: util.Ptr(3), IncrBackups: util.Ptr(-1)},
			expectedErr: "incremental backups retention -1 is invalid, cannot be negative",
		},
		{
			name:        "invalid incremental backups: exceeds full backups",
			policy:      &RetentionPolicy{FullBackups: util.Ptr(3), IncrBackups: util.Ptr(5)},
			expectedErr: "incremental backups retention 5 cannot exceed full backups retention 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.policy.Validate()
			if tt.expectedErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.EqualError(t, err, tt.expectedErr)
			}
		})
	}
}
