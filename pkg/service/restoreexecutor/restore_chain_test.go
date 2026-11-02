package restoreexecutor

import (
	"errors"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRestoreExecutor_Run_ChainOldestFirst(t *testing.T) {
	ctrl := gomock.NewController(t)
	operations, client := expectRestoresInOrder(ctrl, "full", "incr1", "incr2")

	chain, err := NewRestoreExecutor(operations).Run(t.Context(), client,
		testChainRequest("full", false), testChainRequest("incr1", false), testChainRequest("incr2", false))
	require.NoError(t, err)
	require.NoError(t, chain.Wait(t.Context()))
	assert.Equal(t, uint64(3), chain.GetStats().GetRecordsInserted())
}

// Under the unique (CREATE_ONLY) policy a record keeps the first version written, so the
// newest backup goes first.
func TestRestoreExecutor_Run_UniqueChainNewestFirst(t *testing.T) {
	ctrl := gomock.NewController(t)
	operations, client := expectRestoresInOrder(ctrl, "incr2", "incr1", "full")

	chain, err := NewRestoreExecutor(operations).Run(t.Context(), client,
		testChainRequest("full", true), testChainRequest("incr1", true), testChainRequest("incr2", true))
	require.NoError(t, err)
	require.NoError(t, chain.Wait(t.Context()))
}

// disable-reordering keeps a unique chain oldest first.
func TestRestoreExecutor_Run_UniqueChainKeepsOrderWhenReorderingDisabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	operations, client := expectRestoresInOrder(ctrl, "full", "incr1", "incr2")

	requests := []*model.RestoreRequest{
		testChainRequest("full", true), testChainRequest("incr1", true), testChainRequest("incr2", true),
	}
	for _, request := range requests {
		request.DisableReordering = true
	}

	chain, err := NewRestoreExecutor(operations).Run(t.Context(), client, requests[0], requests[1:]...)
	require.NoError(t, err)
	require.NoError(t, chain.Wait(t.Context()))
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

	chain, err := NewRestoreExecutor(operations).Run(t.Context(), client,
		testChainRequest("full", false), testChainRequest("incr1", false))
	require.NoError(t, err)
	require.ErrorIs(t, chain.Wait(t.Context()), wantErr)
}

func TestSequentialRestoreHandler_BeforeFirstRestore(t *testing.T) {
	handler := newSequentialRestoreHandler()

	assert.Zero(t, handler.GetStats().GetRecordsInserted())
	assert.Equal(t, &models.Metrics{}, handler.GetMetrics())
}

func testChainRequest(path string, unique bool) *model.RestoreRequest {
	request := testRestoreRequest()
	request.BackupDataPath = path
	request.Policy.Unique = &unique

	return request
}

func restoreStatsInserted(n int) *models.RestoreStats {
	stats := models.NewRestoreStats()
	for range n {
		stats.IncrRecordsInserted()
	}

	return stats
}
