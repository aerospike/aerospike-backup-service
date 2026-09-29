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
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/syncutil"
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

	handler1 := NewMockCancelableBackupHandler(ctrl)
	handler1.EXPECT().GetStats().Return(models.NewBackupStats()).AnyTimes()
	handler2 := NewMockCancelableBackupHandler(ctrl)
	handler2.EXPECT().GetStats().Return(models.NewBackupStats()).AnyTimes()

	nsRunner.EXPECT().
		Run(gomock.Any(), forNamespace("ns1"), gomock.Any(), gomock.Any()).
		Return(handler1, nil)
	nsRunner.EXPECT().
		Run(gomock.Any(), forNamespace("ns2"), gomock.Any(), gomock.Any()).
		Return(handler2, nil)

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

// A namespace backup that cannot be started fails the routine run instead of waiting for its
// context, which for a scheduled backup is the scheduler's and lives until the service shuts down.
func TestRoutineBackupRunner_Run_ReturnsStartError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	nsRunner := NewMockNamespaceBackupRunner(ctrl)
	resolver := aerospike.NewMockNamespaceResolver(ctrl)
	resolver.EXPECT().
		ResolveNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]string{"ns1"}, nil)

	startErr := errors.New("cluster unreachable")
	nsRunner.EXPECT().
		Run(gomock.Any(), forNamespace("ns1"), gomock.Any(), gomock.Any()).
		Return(nil, startErr)

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

	running := NewMockCancelableBackupHandler(ctrl)
	running.EXPECT().Cancel().Times(1)

	nsRunner.EXPECT().
		Run(gomock.Any(), forNamespace("ns1"), gomock.Any(), gomock.Any()).
		Return(running, nil)
	nsRunner.EXPECT().
		Run(gomock.Any(), forNamespace("ns2"), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("cluster unreachable"))

	runner := NewRoutineBackupRunner(nsRunner, resolver)
	routine := &model.BackupRoutine{Name: "daily", BackupPolicy: &model.BackupPolicy{}}

	op, err := runner.Run(t.Context(), routine, model.BackupRunSpec{Type: model.BackupTypeFull}, slog.Default())
	require.Error(t, err)
	assert.Nil(t, op)
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

// A run that has not started yet and never will is bounded by the caller's context.
func TestRoutineBackupRunner_Run_StartWaitHonorsContext(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	executor := backupexecutor.NewMockBackup(ctrl)
	writer := NewMockBackupWriter(ctrl)
	resolver := aerospike.NewMockNamespaceResolver(ctrl)
	resolver.EXPECT().ResolveNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]string{"ns1"}, nil)

	executor.EXPECT().
		Run(gomock.Any(), gomock.Any(), gomock.Any(), "ns1", gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, _ *model.BackupRoutine, _ model.TimeBounds,
			_, _ string, _ syncutil.Limiter, _ *slog.Logger,
		) (backupexecutor.BackupHandler, error) {
			<-ctx.Done()

			return nil, ctx.Err()
		}).AnyTimes()
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

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
	defer cancel()

	op, err := runner.Run(ctx, routine, model.BackupRunSpec{Type: model.BackupTypeFull}, slog.Default())
	require.Error(t, err)
	assert.Nil(t, op)
}

// forNamespace matches the namespace run of the given namespace.
func forNamespace(namespace string) gomock.Matcher {
	return gomock.Cond(func(run model.NamespaceRun) bool { return run.Namespace == namespace })
}
