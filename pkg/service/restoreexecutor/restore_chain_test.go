package restoreexecutor

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRestoreExecutor_Run_ChainInGivenOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	operations, client := expectRestoresInOrder(ctrl, "full", "incr1", "incr2")

	chain := NewRestoreExecutor(operations).Run(t.Context(), client, slog.Default(),
		testChainRequest("full"), testChainRequest("incr1"), testChainRequest("incr2"))
	require.NoError(t, waitChain(t, chain))
	assert.Equal(t, uint64(3), chain.GetStats().GetRecordsInserted())
}

// expectRestoresInOrder expects one successful restore of each backup at paths, in that order,
// each inserting one record.
func expectRestoresInOrder(
	ctrl *gomock.Controller,
	paths ...string,
) (*storage.MockOperations, *aerospike.MockClient) {
	operations := storage.NewMockOperations(ctrl)
	client := aerospike.NewMockClient(ctrl)

	var calls []any
	for _, path := range paths {
		reader := NewMockStreamingReader(ctrl)
		handler := NewMockRestoreHandler(ctrl)
		handler.EXPECT().Wait(gomock.Any()).Return(nil)
		handler.EXPECT().GetStats().Return(restoreStatsInserted(1)).AnyTimes()

		calls = append(calls,
			operations.EXPECT().
				CreateDirReader(gomock.Any(), gomock.Any(), path, gomock.Any()).
				Return(reader, nil),
			client.EXPECT().
				Restore(gomock.Any(), gomock.Any(), reader).
				Return(handler, nil),
		)
	}
	gomock.InOrder(calls...)

	return operations, client
}

func TestRestoreExecutor_Run_ChainStopsAtFirstFailure(t *testing.T) {
	ctrl := gomock.NewController(t)

	wantErr := errors.New("restore failed")

	operations := storage.NewMockOperations(ctrl)
	reader := NewMockStreamingReader(ctrl)
	operations.EXPECT().
		CreateDirReader(gomock.Any(), gomock.Any(), "full", gomock.Any()).
		Return(reader, nil)

	handler := NewMockRestoreHandler(ctrl)
	handler.EXPECT().Wait(gomock.Any()).Return(wantErr)

	client := aerospike.NewMockClient(ctrl)
	client.EXPECT().Restore(gomock.Any(), gomock.Any(), reader).Return(handler, nil)

	chain := NewRestoreExecutor(operations).Run(t.Context(), client, slog.Default(),
		testChainRequest("full"), testChainRequest("incr1"))
	err := waitChain(t, chain)
	require.ErrorIs(t, err, wantErr)
	assert.ErrorContains(t, err, "backup at full")
}

// A later restore that cannot start fails the chain after the earlier ones have finished.
func TestRestoreExecutor_Run_LaterStartFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	wantErr := errors.New("no such backup")

	operations := storage.NewMockOperations(ctrl)
	reader := NewMockStreamingReader(ctrl)
	handler := NewMockRestoreHandler(ctrl)
	handler.EXPECT().Wait(gomock.Any()).Return(nil)
	client := aerospike.NewMockClient(ctrl)
	gomock.InOrder(
		operations.EXPECT().CreateDirReader(gomock.Any(), gomock.Any(), "full", gomock.Any()).Return(reader, nil),
		client.EXPECT().Restore(gomock.Any(), gomock.Any(), reader).Return(handler, nil),
		operations.EXPECT().CreateDirReader(gomock.Any(), gomock.Any(), "incr1", gomock.Any()).Return(nil, wantErr),
	)

	chain := NewRestoreExecutor(operations).Run(t.Context(), client, slog.Default(),
		testChainRequest("full"), testChainRequest("incr1"))
	require.ErrorIs(t, waitChain(t, chain), wantErr)
}

// A step that finishes as the chain's context is canceled starts no further restore.
func TestRestoreExecutor_Run_CanceledChainStartsNoMore(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctx, cancel := context.WithCancel(t.Context())

	operations := storage.NewMockOperations(ctrl)
	reader := NewMockStreamingReader(ctrl)
	handler := NewMockRestoreHandler(ctrl)
	handler.EXPECT().Wait(gomock.Any()).DoAndReturn(func(context.Context) error {
		cancel()
		return nil
	})
	client := aerospike.NewMockClient(ctrl)
	operations.EXPECT().CreateDirReader(gomock.Any(), gomock.Any(), "full", gomock.Any()).Return(reader, nil)
	client.EXPECT().Restore(gomock.Any(), gomock.Any(), reader).Return(handler, nil)

	chain := NewRestoreExecutor(operations).Run(ctx, client, slog.Default(),
		testChainRequest("full"), testChainRequest("incr1"))
	require.ErrorIs(t, waitChain(t, chain), context.Canceled)
}

func testChainRequest(path string) *model.RestoreRequest {
	request := testRestoreRequest()
	request.BackupDataPath = path

	return request
}

func restoreStatsInserted(n int) *models.RestoreStats {
	stats := models.NewRestoreStats()
	for range n {
		stats.IncrRecordsInserted()
	}

	return stats
}

// waitChain waits for the chain with a deadline: a failed expectation in its background
// goroutine ends that goroutine without finishing the chain, which would otherwise hang Wait.
func waitChain(t *testing.T, chain RestoreHandler) error {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	return chain.Wait(ctx)
}
