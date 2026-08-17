package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type fakeNamespaceResolver struct {
	namespaces []string
	err        error
}

func (f *fakeNamespaceResolver) ResolveNamespaces(
	_ context.Context, _ *model.BackupRoutine, _ *slog.Logger,
) ([]string, error) {
	return f.namespaces, f.err
}

var _ aerospike.NamespaceResolver = (*fakeNamespaceResolver)(nil)

func TestRoutineBackupRunner_Run_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nsRunner := NewMockNamespaceBackupRunner(ctrl)
	resolver := &fakeNamespaceResolver{namespaces: []string{"ns1", "ns2"}}

	handler1 := NewMockCancelableBackupHandler(ctrl)
	handler1.EXPECT().GetStats().Return(models.NewBackupStats()).AnyTimes()
	handler2 := NewMockCancelableBackupHandler(ctrl)
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
	defer ctrl.Finish()

	nsRunner := NewMockNamespaceBackupRunner(ctrl)
	resolver := &fakeNamespaceResolver{err: errors.New("resolve failed")}

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
	defer ctrl.Finish()

	nsRunner := NewMockNamespaceBackupRunner(ctrl)
	resolver := &fakeNamespaceResolver{namespaces: []string{"ns1"}}

	handler := NewMockCancelableBackupHandler(ctrl)
	handler.EXPECT().GetStats().Return(nil).AnyTimes()

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
