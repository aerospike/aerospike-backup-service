package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/require"
)

func TestBackupPolicyConversionIsLossless(t *testing.T) {
	parallel := 4
	parallelWrite := 8
	socketTimeout := int64(5000)
	totalTimeout := int64(10000)
	retryPolicy := &RetryPolicy{MaxRetries: ptr.Of(3)}
	noRecords := true
	noIndexes := false
	noUdfs := true
	bandwidth := int64(50)
	recordsPerSecond := 100
	fileLimit := 1024
	compressionPolicy := &CompressionPolicy{Level: 5}
	sealed := true
	compact := true

	original := &BackupPolicy{
		Parallel:      &parallel,
		ParallelWrite: &parallelWrite,
		SocketTimeout: &socketTimeout,
		TotalTimeout:  &totalTimeout,
		RetryPolicy:   retryPolicy,
		RetentionPolicy: &RetentionPolicy{
			FullBackups: ptr.Of(10),
			IncrBackups: ptr.Of(5),
		},
		NoRecords:          &noRecords,
		NoIndexes:          &noIndexes,
		NoUdfs:             &noUdfs,
		Bandwidth:          &bandwidth,
		RecordsPerSecond:   &recordsPerSecond,
		FileLimit:          &fileLimit,
		EncryptionPolicy:   nil,
		CompressionPolicy:  compressionPolicy,
		Sealed:             &sealed,
		Compact:            &compact,
		MaxConcurrentNodes: ptr.Of(3),
		UseCompression:     ptr.Of(true),
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
			policy:      &RetentionPolicy{FullBackups: ptr.Of(5), IncrBackups: ptr.Of(3)},
			expectedErr: "",
		},
		{
			name:        "valid policy with only full backups",
			policy:      &RetentionPolicy{FullBackups: ptr.Of(2), IncrBackups: nil},
			expectedErr: "",
		},
		{
			name:        "valid policy with only incremental backups set to zero",
			policy:      &RetentionPolicy{FullBackups: ptr.Of(3), IncrBackups: ptr.Of(0)},
			expectedErr: "",
		},
		{
			name:        "invalid full backups: less than 1",
			policy:      &RetentionPolicy{FullBackups: ptr.Of(0), IncrBackups: ptr.Of(1)},
			expectedErr: "full backups retention 0 is invalid, must be at least 1",
		},
		{
			name:        "invalid incremental backups: negative value",
			policy:      &RetentionPolicy{FullBackups: ptr.Of(3), IncrBackups: ptr.Of(-1)},
			expectedErr: "negative value validation error: \"incremental\" -1 invalid, should not be negative number",
		},
		{
			name:        "invalid incremental backups: exceeds full backups",
			policy:      &RetentionPolicy{FullBackups: ptr.Of(3), IncrBackups: ptr.Of(5)},
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

func TestBackupPolicy_Validate(t *testing.T) {
	tests := []struct {
		name        string
		policy      *BackupPolicy
		expectedErr string
	}{
		{
			name:        "valid parallel",
			policy:      &BackupPolicy{Parallel: ptr.Of(1)},
			expectedErr: "",
		},
		{
			name:        "invalid parallel: zero",
			policy:      &BackupPolicy{Parallel: ptr.Of(0)},
			expectedErr: "non-positive value validation error: \"parallel\" 0 invalid, should be positive number",
		},
		{
			name:        "invalid parallel: negative",
			policy:      &BackupPolicy{Parallel: ptr.Of(-1)},
			expectedErr: "non-positive value validation error: \"parallel\" -1 invalid, should be positive number",
		},
		{
			name:        "valid parallel-write",
			policy:      &BackupPolicy{ParallelWrite: ptr.Of(1)},
			expectedErr: "",
		},
		{
			name:        "invalid parallel-write: zero",
			policy:      &BackupPolicy{ParallelWrite: ptr.Of(0)},
			expectedErr: "non-positive value validation error: \"parallel-write\" 0 invalid, should be positive number",
		},
		{
			name:        "invalid parallel-write: negative",
			policy:      &BackupPolicy{ParallelWrite: ptr.Of(-1)},
			expectedErr: "non-positive value validation error: \"parallel-write\" -1 invalid, should be positive number",
		},
		{
			name:        "invalid max concurrent nodes: negative",
			policy:      &BackupPolicy{MaxConcurrentNodes: ptr.Of(-1)},
			expectedErr: "negative value validation error: \"max-concurrent-nodes\" -1 invalid, should not be negative number",
		},
		{
			name:        "nil policy",
			policy:      nil,
			expectedErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.policy.Validate(0)
			if tt.expectedErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.EqualError(t, err, tt.expectedErr)
			}
		})
	}
}
