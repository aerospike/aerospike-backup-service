package service

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/backupexecutor"
	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// On service shutdown (parent ctx canceled) the in-flight backup fails with context.Canceled,
// while OnFail, which deletes the partial backup folder, receives a context that is still alive.
func TestRetryableBackupHandler_OnFailKeepsLiveContextOnShutdown(t *testing.T) {
	ctrl := gomock.NewController(t)

	inner := backupexecutor.NewMockBackupHandler(ctrl)
	inner.EXPECT().Wait(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}).AnyTimes()
	inner.EXPECT().GetStats().Return(models.NewBackupStats()).AnyTimes()

	parent, cancelParent := context.WithCancel(t.Context())

	onFailCtxErr := make(chan error, 1)
	h := newRetryableBackupHandler(parent, models.RetryPolicy{MaxRetries: 0, BaseTimeout: time.Millisecond, Multiplier: 1},
		retryableBackupCallbacks{
			Start:     func(context.Context) (backupexecutor.BackupHandler, error) { return inner, nil },
			OnFail:    func(ctx context.Context) { onFailCtxErr <- ctx.Err() },
			OnSuccess: func(context.Context, *models.BackupStats) error { return nil },
			OnRetry:   func() {},
		}, slog.Default())

	// Simulate SIGTERM: the scheduler context is canceled.
	cancelParent()

	err := h.Wait(t.Context())
	require.Error(t, err)

	select {
	case got := <-onFailCtxErr:
		require.NoError(t, got, "OnFail (folder cleanup) must be able to do I/O after cancellation")
	case <-time.After(2 * time.Second):
		t.Fatal("OnFail was not called")
	}
}
