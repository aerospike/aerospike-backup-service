package scanbackup

import (
	"errors"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
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
	// The backups are restored one after another, the full backup first.
	env.expectRestores(client,
		restoreRequestPathMatcher{expectedPath: fullBackup.Key},
		restoreRequestPathMatcher{expectedPath: incrBackup.Key})

	jobID := env.restoreManager.RestoreByTime(request)
	jobStatus, err := waitForRestore(t, env.restoreManager, jobID)
	require.NoError(t, err)
	assert.Equal(t, model.RestoreSuccess, jobStatus.Status)
}

// Empty backups (no files) are left out of the chain: an empty full backup leaves the chain to
// the incremental after it, and a chain of only empty backups restores nothing and succeeds.
func TestRestoreByTime_SkipsEmptyBackups(t *testing.T) {
	tests := []struct {
		name          string
		fullFiles     uint64
		incrFiles     uint64
		expectRestore []string
	}{
		{name: "no empty backups", fullFiles: 1, incrFiles: 1, expectRestore: []string{"full", "incr"}},
		{name: "empty full backup", fullFiles: 0, incrFiles: 1, expectRestore: []string{"incr"}},
		{name: "only empty backups", fullFiles: 0, incrFiles: 0, expectRestore: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := setupTestRestoreEnv(t)

			now := time.Now()
			fullCreated := now.Add(-2 * time.Hour)
			requestTime := now.Add(-1 * time.Hour)

			request := &model.RestoreTimestampRequest{
				RoutineName:       "test-routine",
				Time:              requestTime,
				DisableReordering: true,
			}
			env.restoreValidator.EXPECT().
				ValidateTimestamp(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			env.mockBackupReader.EXPECT().
				GetBackups(gomock.Any(), fullBackupFilterMatcher{toTime: requestTime}).
				Return([]model.BackupDetails{testBackup("full", fullCreated, tt.fullFiles)}, nil)
			env.mockBackupReader.EXPECT().
				GetBackups(gomock.Any(), incrementalFilterMatcher{fromTime: fullCreated, toTime: requestTime}).
				Return([]model.BackupDetails{testBackup("incr", fullCreated.Add(time.Minute), tt.incrFiles)}, nil)

			client := env.expectSuccessfulClientInteraction(t)
			if len(tt.expectRestore) > 0 {
				chain := make([]any, 0, len(tt.expectRestore))
				for _, path := range tt.expectRestore {
					chain = append(chain, restoreRequestPathMatcher{expectedPath: path})
				}
				env.expectRestores(client, chain...)
			}

			jobStatus, err := waitForRestore(t, env.restoreManager, env.restoreManager.RestoreByTime(request))
			require.NoError(t, err)
			assert.Equal(t, model.RestoreSuccess, jobStatus.Status)
		})
	}
}

// Each backup of a chain is restored with its own compression, so a full and an incremental
// backup compressed differently do not share one policy.
func TestRestoreByTime_CompressionPerBackup(t *testing.T) {
	env := setupTestRestoreEnv(t)
	now := time.Now()
	full := testBackup("full", now.Add(-2*time.Hour), 1)
	full.Compression = model.CompressionModeZSTD
	incr := testBackup("incr", now.Add(-time.Hour), 1)
	incr.Compression = model.CompressionModeNone
	request := expectChainOfTwo(t, env, full, incr)

	env.expectRestores(gomock.Any(),
		restoreRequestCompressionMatcher{full.Key, model.CompressionModeZSTD},
		restoreRequestCompressionMatcher{incr.Key, model.CompressionModeNone})

	jobStatus, err := waitForRestore(t, env.restoreManager, env.restoreManager.RestoreByTime(request))
	require.NoError(t, err)
	assert.Equal(t, model.RestoreSuccess, jobStatus.Status)
}

// A chain that fails fails the job with the chain's error.
func TestRestoreByTime_ChainFailureFailsJob(t *testing.T) {
	env := setupTestRestoreEnv(t)
	now := time.Now()
	request := expectChainOfTwo(t, env,
		testBackup("full", now.Add(-2*time.Hour), 1), testBackup("incr", now.Add(-time.Hour), 1))

	restoreErr := errors.New("failed to restore backup at incr")
	env.mockRestore.EXPECT().
		Run(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(failedRestore{err: restoreErr}, nil)

	jobStatus, err := waitForRestore(t, env.restoreManager, env.restoreManager.RestoreByTime(request))
	require.NoError(t, err)
	assert.Equal(t, model.RestoreFailure, jobStatus.Status)
	require.ErrorIs(t, jobStatus.Error, restoreErr)
}

// expectChainOfTwo returns a request to restore, with reordering disabled, the chain of a full and
// an incremental backup, and sets the expectations of finding that chain and passing validation.
func expectChainOfTwo(
	t *testing.T, env *testRestoreEnv, full, incr model.BackupDetails,
) *model.RestoreTimestampRequest {
	t.Helper()

	request := &model.RestoreTimestampRequest{
		RoutineName: "test-routine", Time: incr.Created.Add(time.Minute), DisableReordering: true,
	}

	env.mockBackupReader.EXPECT().
		GetBackups(gomock.Any(), fullBackupFilterMatcher{toTime: request.Time}).
		Return([]model.BackupDetails{full}, nil)
	env.mockBackupReader.EXPECT().
		GetBackups(gomock.Any(), incrementalFilterMatcher{fromTime: full.Created, toTime: request.Time}).
		Return([]model.BackupDetails{incr}, nil)
	env.restoreValidator.EXPECT().
		ValidateTimestamp(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	env.expectSuccessfulClientInteraction(t)

	return request
}

// testBackup returns the details of a backup of namespace ns1 in local storage.
func testBackup(key string, created time.Time, files uint64) model.BackupDetails {
	return model.BackupDetails{
		BackupMetadata: model.BackupMetadata{Created: created, Namespace: "ns1", FileCount: files},
		Key:            key,
		Storage:        &model.LocalStorage{},
	}
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

	jobID := env.restoreManager.RestoreByTime(request)
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
				// Incrementals with the same timestamp as full are skipped, so only full runs, with
				// the backup's compression.
				env.mockRestore.EXPECT().
					Run(gomock.Any(), client, restoreRequestCompressionMatcher{backup.Key, tt.backupCompression}).
					Return(env.expectDefaultRestoreHandler(), nil).Times(1)
			} else {
				env.restoreValidator.EXPECT().
					ValidateTimestamp(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errors.New("validator failed")).
					AnyTimes()
			}

			jobID := env.restoreManager.RestoreByTime(request)

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

func TestRestoreByTime_CumulativeIncrementals(t *testing.T) {
	env := setupTestRestoreEnv(t)

	now := time.Now()
	fullCreated := now.Add(-3 * time.Hour)
	incr1Created := now.Add(-2 * time.Hour)
	incr2Created := now.Add(-1 * time.Hour)
	requestTime := now

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
	incr1Backup := model.BackupDetails{
		BackupMetadata: model.BackupMetadata{
			Created:   incr1Created,
			From:      fullCreated,
			Namespace: "ns1",
			FileCount: 1,
		},
		Key:     "incr1",
		Storage: &model.LocalStorage{},
	}
	incr2Backup := model.BackupDetails{
		BackupMetadata: model.BackupMetadata{
			Created:   incr2Created,
			From:      fullCreated,
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
	// The chain is restored full backup first, then the incremental. Since incr2 covers the
	// same period as incr1, incr1 is skipped.
	env.expectRestores(client,
		restoreRequestPathMatcher{expectedPath: fullBackup.Key},
		restoreRequestPathMatcher{expectedPath: incr2Backup.Key})

	jobID := env.restoreManager.RestoreByTime(request)
	jobStatus, err := waitForRestore(t, env.restoreManager, jobID)
	require.NoError(t, err)
	assert.Equal(t, model.RestoreSuccess, jobStatus.Status)
}

// TestRestoreByTime_HandsChainOldestFirst checks that the time runner hands the restore each
// namespace's chain oldest first, as found, with its namespace and the request's
// disable-reordering: the order the backups are applied in, and the policy that order depends on,
// are the restore's to decide. The destination's record count is not asked for.
func TestRestoreByTime_HandsChainOldestFirst(t *testing.T) {
	tests := []struct {
		name              string
		disableReordering bool
	}{
		{name: "reordering allowed"},
		{name: "reordering disabled", disableReordering: true},
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
				RoutineName:        "test-routine",
				Time:               requestTime,
				DisableReordering:  tt.disableReordering,
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

			chain := make([]any, 0, 3)
			for _, path := range []string{"full", "incr1", "incr2"} {
				chain = append(chain, restoreRequestChainMatcher{path, "ns1", tt.disableReordering})
			}
			env.expectRestores(client, chain...)

			jobID := env.restoreManager.RestoreByTime(request)
			jobStatus, err := waitForRestore(t, env.restoreManager, jobID)
			require.NoError(t, err)
			assert.Equal(t, model.RestoreSuccess, jobStatus.Status)
		})
	}
}
