package service

import (
	"context"
	"errors"
	"log/slog"
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

func TestRoutineBackupRunner_Run_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	nsRunner := NewMockNamespaceBackupRunner(ctrl)
	resolver := aerospike.NewMockNamespaceResolver(ctrl)
	resolver.EXPECT().
		ResolveNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]string{"ns1", "ns2"}, nil)

	handler1 := NewMockNamespaceBackupHandler(ctrl)
	handler1.EXPECT().GetStats().Return(models.NewBackupStats()).AnyTimes()
	handler2 := NewMockNamespaceBackupHandler(ctrl)
	handler2.EXPECT().GetStats().Return(models.NewBackupStats()).AnyTimes()

	nsRunner.EXPECT().
		Run(gomock.Any(), gomock.Any(), "ns1", gomock.Any(), gomock.Any(), gomock.Any()).
		Return(handler1)
	nsRunner.EXPECT().
		Run(gomock.Any(), gomock.Any(), "ns2", gomock.Any(), gomock.Any(), gomock.Any()).
		Return(handler2)

	runner := NewRoutineBackupRunner(nsRunner, resolver)
	routine := &model.BackupRoutine{
		Name:         "daily",
		BackupPolicy: &model.BackupPolicy{},
	}

	op, err := runner.Run(t.Context(), routine, model.BackupRunSpec{Type: model.BackupTypeFull}, slog.Default())
	require.NoError(t, err)
	require.NotNil(t, op)
	assert.Len(t, op.handlers, 2)
}

func TestRoutineBackupRunner_Run_ResolverError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	nsRunner := NewMockNamespaceBackupRunner(ctrl)
	resolver := aerospike.NewMockNamespaceResolver(ctrl)
	resolver.EXPECT().
		ResolveNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("resolve failed"))

	runner := NewRoutineBackupRunner(nsRunner, resolver)
	routine := &model.BackupRoutine{Name: "daily", BackupPolicy: &model.BackupPolicy{}}

	op, err := runner.Run(t.Context(), routine, model.BackupRunSpec{Type: model.BackupTypeFull}, slog.Default())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolve failed")
	assert.Nil(t, op)
}

func TestRoutineBackupRunner_Run_WaitUntilStartedTimesOut(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	nsRunner := NewMockNamespaceBackupRunner(ctrl)
	resolver := aerospike.NewMockNamespaceResolver(ctrl)
	resolver.EXPECT().
		ResolveNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]string{"ns1"}, nil)

	handler := NewMockNamespaceBackupHandler(ctrl)
	handler.EXPECT().GetStats().Return(nil).AnyTimes()
	handler.EXPECT().Done().Return(neverDone()).AnyTimes()
	handler.EXPECT().Cancel().AnyTimes()

	nsRunner.EXPECT().
		Run(gomock.Any(), gomock.Any(), "ns1", gomock.Any(), gomock.Any(), gomock.Any()).
		Return(handler)

	runner := NewRoutineBackupRunner(nsRunner, resolver)
	routine := &model.BackupRoutine{Name: "daily", BackupPolicy: &model.BackupPolicy{}}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
	defer cancel()

	op, err := runner.Run(ctx, routine, model.BackupRunSpec{Type: model.BackupTypeFull}, slog.Default())
	require.Error(t, err)
	assert.Nil(t, op)
}

// neverDone stands for a run that is still going.
func neverDone() <-chan struct{} {
	return make(chan struct{})
}

// closedDone stands for a run that has already ended.
func closedDone() <-chan struct{} {
	done := make(chan struct{})
	close(done)

	return done
}

// A namespace backup that fails to start permanently never publishes statistics. Run must
// report that failure instead of waiting for its context, which for a scheduled backup is the
// scheduler's and lives until the service shuts down.
func TestRoutineBackupRunner_Run_ReturnsStartError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	nsRunner := NewMockNamespaceBackupRunner(ctrl)
	resolver := aerospike.NewMockNamespaceResolver(ctrl)
	resolver.EXPECT().
		ResolveNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]string{"ns1"}, nil)

	startErr := errors.New("cluster unreachable")
	handler := NewMockNamespaceBackupHandler(ctrl)
	handler.EXPECT().GetStats().Return(nil).AnyTimes()
	handler.EXPECT().Done().Return(closedDone()).AnyTimes()
	handler.EXPECT().Err().Return(startErr).AnyTimes()
	handler.EXPECT().Cancel().AnyTimes()

	nsRunner.EXPECT().
		Run(gomock.Any(), gomock.Any(), "ns1", gomock.Any(), gomock.Any(), gomock.Any()).
		Return(handler)

	runner := NewRoutineBackupRunner(nsRunner, resolver)
	routine := &model.BackupRoutine{Name: "daily", BackupPolicy: &model.BackupPolicy{}}

	const deadline = 700 * time.Millisecond // safety bound only; Run must return well before it
	ctx, cancel := context.WithTimeout(t.Context(), deadline)
	defer cancel()

	started := time.Now()
	op, err := runner.Run(ctx, routine, model.BackupRunSpec{Type: model.BackupTypeFull}, slog.Default())

	require.ErrorIs(t, err, startErr)
	assert.Nil(t, op)
	assert.Less(t, time.Since(started), deadline/2, "Run must fail fast instead of waiting for the context")
}

// The namespaces that did start have nobody left to wait for them once Run gives up, so they
// are canceled rather than left writing into the abandoned run's folder.
func TestRoutineBackupRunner_Run_CancelsStartedNamespacesOnFailure(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	nsRunner := NewMockNamespaceBackupRunner(ctrl)
	resolver := aerospike.NewMockNamespaceResolver(ctrl)
	resolver.EXPECT().
		ResolveNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]string{"ns1", "ns2"}, nil)

	running := NewMockNamespaceBackupHandler(ctrl)
	running.EXPECT().GetStats().Return(models.NewBackupStats()).AnyTimes()
	running.EXPECT().Done().Return(neverDone()).AnyTimes()
	running.EXPECT().Cancel().MinTimes(1)

	failed := NewMockNamespaceBackupHandler(ctrl)
	failed.EXPECT().GetStats().Return(nil).AnyTimes()
	failed.EXPECT().Done().Return(closedDone()).AnyTimes()
	failed.EXPECT().Err().Return(errors.New("cluster unreachable")).AnyTimes()
	failed.EXPECT().Cancel().MinTimes(1)

	nsRunner.EXPECT().
		Run(gomock.Any(), gomock.Any(), "ns1", gomock.Any(), gomock.Any(), gomock.Any()).
		Return(running)
	nsRunner.EXPECT().
		Run(gomock.Any(), gomock.Any(), "ns2", gomock.Any(), gomock.Any(), gomock.Any()).
		Return(failed)

	runner := NewRoutineBackupRunner(nsRunner, resolver)
	routine := &model.BackupRoutine{Name: "daily", BackupPolicy: &model.BackupPolicy{}}

	op, err := runner.Run(t.Context(), routine, model.BackupRunSpec{Type: model.BackupTypeFull}, slog.Default())
	require.Error(t, err)
	assert.Nil(t, op)
}

// A run that finishes between two polls is not a failure: the wait ends with its outcome.
func TestRoutineBackupRunner_Run_AcceptsRunFinishedBeforeStatsAppeared(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	nsRunner := NewMockNamespaceBackupRunner(ctrl)
	resolver := aerospike.NewMockNamespaceResolver(ctrl)
	resolver.EXPECT().
		ResolveNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]string{"ns1"}, nil)

	handler := NewMockNamespaceBackupHandler(ctrl)
	handler.EXPECT().GetStats().Return(nil).AnyTimes()
	handler.EXPECT().Done().Return(closedDone()).AnyTimes()
	handler.EXPECT().Err().Return(nil).AnyTimes()

	nsRunner.EXPECT().
		Run(gomock.Any(), gomock.Any(), "ns1", gomock.Any(), gomock.Any(), gomock.Any()).
		Return(handler)

	runner := NewRoutineBackupRunner(nsRunner, resolver)
	routine := &model.BackupRoutine{Name: "daily", BackupPolicy: &model.BackupPolicy{}}

	op, err := runner.Run(t.Context(), routine, model.BackupRunSpec{Type: model.BackupTypeFull}, slog.Default())
	require.NoError(t, err)
	require.NotNil(t, op)
}

// The same through the real handler: a backup executor that always fails to start leaves the
// statistics nil for good, and Run reports the executor's error rather than blocking.
func TestRoutineBackupRunner_Run_ReturnsStartErrorFromExecutor(t *testing.T) {
	t.Parallel()

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
	writer.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

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
