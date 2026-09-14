package service

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/backupexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/optional"
	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// When a namespace backup fails to start (cluster unreachable, no storage writer) Run must
// return that error promptly. Waiting for the run context instead means waiting for the
// long-lived scheduler context, which holds the routine's admission reservation indefinitely.
func TestRoutineBackupRunner_Run_ReturnsStartError(t *testing.T) {
	t.Parallel()
	t.Skip("BKRS-407: waitUntilBackupStarted polls GetStats only and blocks until the context ends")

	ctrl := gomock.NewController(t)
	executor := backupexecutor.NewMockBackup(ctrl)
	writer := NewMockBackupWriter(ctrl)
	resolver := aerospike.NewMockNamespaceResolver(ctrl)
	resolver.EXPECT().ResolveNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]string{"ns1"}, nil)

	startErr := errors.New("cluster unreachable")
	executor.EXPECT().
		Run(gomock.Any(), gomock.Any(), gomock.Any(), "ns1", gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, startErr)

	nsRunner := NewNamespaceBackupRunner(executor, writer, NewPathService(nil))
	runner := NewRoutineBackupRunner(nsRunner, resolver)
	routine := &model.BackupRoutine{
		Name:          "daily",
		SourceCluster: &model.AerospikeCluster{},
		BackupPolicy: &model.BackupPolicy{
			RetryPolicy: &model.RetryPolicy{MaxRetries: optional.Of(0)},
		},
	}

	const deadline = 700 * time.Millisecond // safety bound only; Run must return well before it
	ctx, cancel := context.WithTimeout(t.Context(), deadline)
	defer cancel()

	started := time.Now()
	op, err := runner.Run(ctx, routine, model.BackupRunSpec{Type: model.BackupTypeFull}, slog.Default())

	require.ErrorIs(t, err, startErr)
	assert.Nil(t, op)
	assert.Less(t, time.Since(started), deadline/2, "Run must fail fast instead of waiting for the context")
}

// When one namespace of a multi-namespace routine fails, its cleanup must remove only that
// namespace's folder: the timestamp folder <routine>/backup/<ts> is shared with the sibling
// namespaces of the same run, which may still be writing into it.
func TestNamespaceBackupRunner_FailureDeletesOnlyItsOwnFolder(t *testing.T) {
	t.Parallel()
	t.Skip("BKRS-405: OnFail deletes the whole shared timestamp folder")

	ctrl := gomock.NewController(t)
	executor := backupexecutor.NewMockBackup(ctrl)
	writer := NewMockBackupWriter(ctrl)
	nsRunner := NewNamespaceBackupRunner(executor, writer, NewPathService(nil))

	start := time.UnixMilli(1700000000000)
	sharedTimestampDir := "multi/backup/1700000000000"
	folderA := sharedTimestampDir + "/data/nsA"
	folderB := sharedTimestampDir + "/data/nsB"

	routine := &model.BackupRoutine{
		Name:          "multi",
		SourceCluster: &model.AerospikeCluster{},
		BackupPolicy: &model.BackupPolicy{
			RetryPolicy: &model.RetryPolicy{MaxRetries: optional.Of(0)},
		},
	}

	// nsB is a long-running backup that is still in flight when nsA fails.
	bStillRunning := make(chan struct{})
	cleanupDone := make(chan struct{})
	handlerB := backupexecutor.NewMockBackupHandler(ctrl)
	statsB := models.NewBackupStats()
	statsB.TotalRecords.Store(10)
	statsB.ReadRecords.Add(10)
	handlerB.EXPECT().GetStats().Return(statsB).AnyTimes()
	handlerB.EXPECT().Wait(gomock.Any()).DoAndReturn(func(context.Context) error {
		close(bStillRunning)
		<-cleanupDone // keep "writing" until nsA's cleanup has run
		return nil
	})
	executor.EXPECT().
		Run(gomock.Any(), routine, gomock.Any(), "nsB", folderB, gomock.Any(), gomock.Any()).
		Return(handlerB, nil)

	// nsA fails as soon as nsB is known to be running.
	handlerA := backupexecutor.NewMockBackupHandler(ctrl)
	handlerA.EXPECT().GetStats().Return(models.NewBackupStats()).AnyTimes()
	handlerA.EXPECT().Wait(gomock.Any()).DoAndReturn(func(context.Context) error {
		<-bStillRunning
		return errors.New("nsA scan failed")
	})
	executor.EXPECT().
		Run(gomock.Any(), routine, gomock.Any(), "nsA", folderA, gomock.Any(), gomock.Any()).
		Return(handlerA, nil)

	var (
		mu      sync.Mutex
		deleted []string
		written []string
	)
	writer.EXPECT().Delete(gomock.Any(), routine, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ *model.BackupRoutine, path string) error {
			mu.Lock()
			deleted = append(deleted, path)
			mu.Unlock()
			close(cleanupDone)
			return nil
		})
	writer.EXPECT().WriteBackupMetadata(gomock.Any(), routine, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ *model.BackupRoutine, folder string, _ model.BackupMetadata) error {
			mu.Lock()
			written = append(written, folder)
			mu.Unlock()
			return nil
		})

	spec := model.BackupRunSpec{Type: model.BackupTypeFull, StartTime: start}
	hA := nsRunner.Run(t.Context(), routine, "nsA", spec, nil, slog.Default())
	hB := nsRunner.Run(t.Context(), routine, "nsB", spec, nil, slog.Default())

	require.Error(t, hA.Wait(t.Context()))
	require.NoError(t, hB.Wait(t.Context()))

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []string{folderA}, deleted, "cleanup is scoped to the failed namespace")
	require.Equal(t, []string{folderB}, written, "the sibling namespace completes untouched")
}
