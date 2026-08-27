package service

import (
	"errors"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// TestRestoreByTime_UsesLastFullBackupAsBase verifies that restore-by-time uses the last full
// backup at or before request.Time as the base and fetches incrementals from that full's Created
// time up to request.Time (i.e. restore chain is based on the correct "first" backup).
// Mocks return data only when called with the expected filters.
func TestRestoreByTime_UsesLastFullBackupAsBase(t *testing.T) {
	env := setupTestRestoreEnv(t)

	// All times in the past: full backup T1, request time T2, incremental created between T1 and T2.
	now := time.Now()
	fullCreated := now.Add(-2 * time.Hour)
	requestTime := now.Add(-1 * time.Hour)
	incrCreated := fullCreated.Add(30 * time.Minute)

	request := &model.RestoreTimestampRequest{
		DestinationCluster: model.AerospikeCluster{},
		Policy:             model.RestorePolicy{},
		RoutineName:        "test-routine",
		Time:               requestTime,
		DisableReordering:  true,
	}

	fullBackup := model.BackupDetails{
		BackupMetadata: model.BackupMetadata{
			Created:   fullCreated,
			Namespace: "ns1",
			FileCount: 1,
		},
		Key:     "full",
		Storage: &model.LocalStorage{},
	}
	incrBackup := model.BackupDetails{
		BackupMetadata: model.BackupMetadata{
			Created:   incrCreated,
			Namespace: "ns1",
			FileCount: 1,
		},
		Key:     "incr",
		Storage: &model.LocalStorage{},
	}

	env.restoreValidator.EXPECT().
		ValidateTimestamp(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	gomock.InOrder(
		env.mockBackupReader.EXPECT().
			GetBackups(gomock.Any(), fullBackupFilterMatcher{toTime: requestTime}).
			Return([]model.BackupDetails{fullBackup}, nil),
		env.mockBackupReader.EXPECT().
			GetBackups(gomock.Any(), incrementalFilterMatcher{fromTime: fullCreated, toTime: requestTime}).
			Return([]model.BackupDetails{incrBackup}, nil),
	)

	client := env.expectSuccessfulClientInteraction(t)
	// Restore runs executor once per backup in chain (full then incremental).
	gomock.InOrder(
		env.mockRestore.EXPECT().
			Run(gomock.Any(), client, restoreRequestPathMatcher{expectedPath: fullBackup.Key}).
			Return(env.expectDefaultRestoreHandler(), nil),
		env.mockRestore.EXPECT().
			Run(gomock.Any(), client, restoreRequestPathMatcher{expectedPath: incrBackup.Key}).
			Return(env.expectDefaultRestoreHandler(), nil),
	)

	jobID, err := env.restoreManager.RestoreByTime(t.Context(), request)
	require.NoError(t, err)
	jobStatus, err := waitForRestore(t, env.restoreManager, jobID)
	require.NoError(t, err)
	assert.Equal(t, model.RestoreSuccess, jobStatus.Status)
}

func TestRestoreByTime_SelectsLatestFullPerNamespace(t *testing.T) {
	env := setupTestRestoreEnv(t)

	now := time.Now()
	fullAt11 := now.Add(-2 * time.Hour)
	fullAt12 := now.Add(-1 * time.Hour)
	requestTime := fullAt12.Add(30 * time.Minute)

	request := &model.RestoreTimestampRequest{
		DestinationCluster: model.AerospikeCluster{},
		Policy:             model.RestorePolicy{},
		RoutineName:        "test-routine",
		Time:               requestTime,
		DisableReordering:  true,
	}

	smallAt11 := model.BackupDetails{
		BackupMetadata: model.BackupMetadata{
			Created:   fullAt11,
			Namespace: "small-ns",
			FileCount: 1,
		},
		Key:     "full/small/11",
		Storage: &model.LocalStorage{},
	}
	smallAt12 := model.BackupDetails{
		BackupMetadata: model.BackupMetadata{
			Created:   fullAt12,
			Namespace: "small-ns",
			FileCount: 1,
		},
		Key:     "full/small/12",
		Storage: &model.LocalStorage{},
	}
	largeAt11 := model.BackupDetails{
		BackupMetadata: model.BackupMetadata{
			Created:   fullAt11,
			Namespace: "large-ns",
			FileCount: 1,
		},
		Key:     "full/large/11",
		Storage: &model.LocalStorage{},
	}

	env.restoreValidator.EXPECT().
		ValidateTimestamp(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()

	gomock.InOrder(
		env.mockBackupReader.EXPECT().
			GetBackups(gomock.Any(), fullBackupFilterMatcher{toTime: requestTime}).
			Return([]model.BackupDetails{smallAt11, smallAt12, largeAt11}, nil),
		env.mockBackupReader.EXPECT().
			GetBackups(gomock.Any(), incrementalFilterMatcher{fromTime: fullAt11, toTime: requestTime}).
			Return(nil, nil),
	)

	client := env.expectSuccessfulClientInteraction(t)
	env.mockRestore.EXPECT().
		Run(gomock.Any(), client, restoreRequestPathMatcher{expectedPath: smallAt12.Key}).
		Return(env.expectDefaultRestoreHandler(), nil).
		Times(1)
	env.mockRestore.EXPECT().
		Run(gomock.Any(), client, restoreRequestPathMatcher{expectedPath: largeAt11.Key}).
		Return(env.expectDefaultRestoreHandler(), nil).
		Times(1)

	jobID, err := env.restoreManager.RestoreByTime(t.Context(), request)
	require.NoError(t, err)
	jobStatus, err := waitForRestore(t, env.restoreManager, jobID)
	require.NoError(t, err)
	assert.Equal(t, model.RestoreSuccess, jobStatus.Status)
}

func TestRestoreByTime_CompressionAndEncryptionHandling(t *testing.T) {
	tests := []struct {
		name              string
		policy            model.RestorePolicy
		backupEncryption  model.EncryptionMode
		backupCompression model.CompressionMode
		shouldSucceed     bool
	}{
		{
			name:              "sets compression policy from backup",
			policy:            model.RestorePolicy{},
			backupCompression: model.CompressionModeZSTD,
			shouldSucceed:     true,
		},
		{
			name:             "fails when encrypted backup has no policy",
			policy:           model.RestorePolicy{},
			backupEncryption: model.EncryptionModeAES128,
			shouldSucceed:    false,
		},
		{
			name: "fails when encryption mode mismatches",
			policy: model.RestorePolicy{
				EncryptionPolicy: &model.EncryptionPolicy{Mode: model.EncryptionModeAES256},
			},
			backupEncryption: model.EncryptionModeAES128,
			shouldSucceed:    false,
		},
		{
			name: "fails when encryption key is missing",
			policy: model.RestorePolicy{
				EncryptionPolicy: &model.EncryptionPolicy{Mode: model.EncryptionModeAES128},
			},
			backupEncryption: model.EncryptionModeAES128,
			shouldSucceed:    false,
		},
		{
			name: "succeeds with valid encryption policy",
			policy: model.RestorePolicy{
				EncryptionPolicy: &model.EncryptionPolicy{
					Mode:   model.EncryptionModeAES128,
					KeyEnv: "AES_KEY",
				},
			},
			backupEncryption: model.EncryptionModeAES128,
			shouldSucceed:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := setupTestRestoreEnv(t)

			request := &model.RestoreTimestampRequest{
				DestinationCluster: model.AerospikeCluster{},
				Policy:             tt.policy,
				RoutineName:        "test-routine",
				Time:               time.Now(),
				DisableReordering:  true,
			}

			backup := model.BackupDetails{
				BackupMetadata: model.BackupMetadata{
					Created:     time.Now().Add(-1 * time.Hour),
					Namespace:   "ns1",
					Encryption:  tt.backupEncryption,
					Compression: tt.backupCompression,
					FileCount:   1,
				},
				Key:     "backup/test",
				Storage: &model.LocalStorage{},
			}

			env.mockBackupReader.EXPECT().
				GetBackups(gomock.Any(), gomock.Any()).
				Return([]model.BackupDetails{backup}, nil).
				Times(2)

			client := env.expectSuccessfulClientInteraction(t)
			if tt.shouldSucceed {
				env.restoreValidator.EXPECT().
					ValidateTimestamp(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).
					AnyTimes()
				// Incrementals with the same timestamp as full are skipped, so only full runs.
				env.mockRestore.EXPECT().
					Run(gomock.Any(), client, gomock.Any()).
					Return(env.expectDefaultRestoreHandler(), nil).Times(1)
			} else {
				env.restoreValidator.EXPECT().
					ValidateTimestamp(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errors.New("validator failed")).
					AnyTimes()
			}

			jobID, err := env.restoreManager.RestoreByTime(t.Context(), request)
			require.NoError(t, err)

			jobStatus, err := waitForRestore(t, env.restoreManager, jobID)
			require.NoError(t, err)

			if tt.shouldSucceed {
				assert.Equal(t, model.RestoreSuccess, jobStatus.Status)
			} else {
				assert.Equal(t, model.RestoreFailure, jobStatus.Status)
				require.Error(t, jobStatus.Error)
			}
		})
	}
}

func TestRestoreByTime_OrderScenarios(t *testing.T) {
	tests := []struct {
		name          string
		recordCount   int64
		unique        *bool
		expectedOrder []string // backup keys in expected order
	}{
		{
			name:          "Scenario 1: Empty namespace -> Reverse order",
			recordCount:   0,
			unique:        nil, // unique will be set to true by the code
			expectedOrder: []string{"incr2", "incr1", "full"},
		},
		{
			name:          "Scenario 2: Non-empty namespace, unique=false -> Chronological order",
			recordCount:   100,
			unique:        ptr.Of(false),
			expectedOrder: []string{"full", "incr1", "incr2"},
		},
		{
			name:          "Scenario 3: Non-empty namespace, unique=true -> Reverse order",
			recordCount:   100,
			unique:        ptr.Of(true),
			expectedOrder: []string{"incr2", "incr1", "full"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := setupTestRestoreEnv(t)

			now := time.Now()
			fullCreated := now.Add(-3 * time.Hour)
			incr1Created := now.Add(-2 * time.Hour)
			incr2Created := now.Add(-1 * time.Hour)
			requestTime := now

			request := &model.RestoreTimestampRequest{
				DestinationCluster: model.AerospikeCluster{},
				Policy: model.RestorePolicy{
					Unique: tt.unique,
				},
				RoutineName:       "test-routine",
				Time:              requestTime,
				DisableReordering: false, // Ensure reordering logic is enabled
			}

			fullBackup := model.BackupDetails{
				BackupMetadata: model.BackupMetadata{
					Created:   fullCreated,
					Namespace: "ns1",
					FileCount: 1,
				},
				Key:     "full",
				Storage: &model.LocalStorage{},
			}
			incr1Backup := model.BackupDetails{
				BackupMetadata: model.BackupMetadata{
					Created:   incr1Created,
					Namespace: "ns1",
					FileCount: 1,
				},
				Key:     "incr1",
				Storage: &model.LocalStorage{},
			}
			incr2Backup := model.BackupDetails{
				BackupMetadata: model.BackupMetadata{
					Created:   incr2Created,
					Namespace: "ns1",
					FileCount: 1,
				},
				Key:     "incr2",
				Storage: &model.LocalStorage{},
			}

			env.restoreValidator.EXPECT().
				ValidateTimestamp(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			gomock.InOrder(
				env.mockBackupReader.EXPECT().
					GetBackups(gomock.Any(), fullBackupFilterMatcher{toTime: requestTime}).
					Return([]model.BackupDetails{fullBackup}, nil),
				env.mockBackupReader.EXPECT().
					GetBackups(gomock.Any(), incrementalFilterMatcher{fromTime: fullCreated, toTime: requestTime}).
					Return([]model.BackupDetails{incr1Backup, incr2Backup}, nil),
			)

			client := env.expectSuccessfulClientInteraction(t)

			if tt.unique == nil || !*tt.unique {
				env.infoGetter.EXPECT().
					GetRecordCount(gomock.Any(), "ns1", gomock.Any()).
					Return(uint64(tt.recordCount), nil)
			}

			call1 := env.mockRestore.EXPECT().
				Run(gomock.Any(), client, restoreRequestPathMatcher{expectedPath: tt.expectedOrder[0]}).
				Return(env.expectDefaultRestoreHandler(), nil)
			call2 := env.mockRestore.EXPECT().
				Run(gomock.Any(), client, restoreRequestPathMatcher{expectedPath: tt.expectedOrder[1]}).
				Return(env.expectDefaultRestoreHandler(), nil)
			call3 := env.mockRestore.EXPECT().
				Run(gomock.Any(), client, restoreRequestPathMatcher{expectedPath: tt.expectedOrder[2]}).
				Return(env.expectDefaultRestoreHandler(), nil)

			gomock.InOrder(call1, call2, call3)

			jobID, err := env.restoreManager.RestoreByTime(t.Context(), request)
			require.NoError(t, err)
			jobStatus, err := waitForRestore(t, env.restoreManager, jobID)
			require.NoError(t, err)
			assert.Equal(t, model.RestoreSuccess, jobStatus.Status)
		})
	}
}
