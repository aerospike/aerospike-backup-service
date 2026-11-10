package restoreexecutor

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestInRestoreOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		paths             []string
		unique            *bool
		disableReordering bool
		// recordCount is the destination's record count, nil when it must not be asked for.
		recordCount *uint64
		wantOrder   []string
		wantUnique  bool
	}{
		{name: "chain into a namespace with records", paths: []string{"full", "incr1", "incr2"},
			recordCount: new(uint64(100)), wantOrder: []string{"full", "incr1", "incr2"}},
		{name: "chain into an empty namespace", paths: []string{"full", "incr1", "incr2"},
			recordCount: new(uint64(0)), wantOrder: []string{"incr2", "incr1", "full"}, wantUnique: true},
		{name: "unique chain", paths: []string{"full", "incr1", "incr2"}, unique: new(true),
			wantOrder: []string{"incr2", "incr1", "full"}, wantUnique: true},
		{name: "unique chain, reordering disabled", paths: []string{"full", "incr1", "incr2"}, unique: new(true),
			disableReordering: true, wantOrder: []string{"full", "incr1", "incr2"}, wantUnique: true},
		{name: "reordering disabled into an empty namespace", paths: []string{"full", "incr1"},
			disableReordering: true, wantOrder: []string{"full", "incr1"}},
		{name: "single backup", paths: []string{"full"}, wantOrder: []string{"full"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			client := aerospike.NewMockClient(ctrl)
			if tt.recordCount != nil {
				// The emptiness is checked on the namespace the records are written to.
				info := aerospike.NewMockInfoGetter(ctrl)
				info.EXPECT().GetRecordCount(gomock.Any(), "dest-ns", gomock.Any()).Return(*tt.recordCount, nil)
				client.EXPECT().InfoClient().Return(info)
			}

			requests := make([]*model.RestoreRequest, 0, len(tt.paths))
			for _, path := range tt.paths {
				request := testRestoreRequest()
				request.BackupDataPath = path
				request.Namespace = "source-ns"
				request.Policy.Unique = tt.unique
				request.DisableReordering = tt.disableReordering
				requests = append(requests, request)
			}

			ordered, err := inRestoreOrder(t.Context(), client, slog.Default(), requests)

			require.NoError(t, err)
			order := make([]string, 0, len(ordered))
			for _, request := range ordered {
				order = append(order, request.BackupDataPath)
				assert.Equal(t, tt.wantUnique, request.Policy.IsUniqueOrDefault(), request.BackupDataPath)
			}
			assert.Equal(t, tt.wantOrder, order)
			assert.Equal(t, tt.unique, requests[0].Policy.Unique, "the given requests are left as they were")
		})
	}
}

func TestInRestoreOrder_RecordCountFails(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	info := aerospike.NewMockInfoGetter(ctrl)
	info.EXPECT().GetRecordCount(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint64(0), errors.New("timeout"))
	client := aerospike.NewMockClient(ctrl)
	client.EXPECT().InfoClient().Return(info)

	_, err := inRestoreOrder(t.Context(), client, slog.Default(),
		[]*model.RestoreRequest{testRestoreRequest(), testRestoreRequest()})

	require.ErrorContains(t, err, "failed to determine if namespace dest-ns is empty: timeout")
}

// TestRestoreExecutor_Run_EmptyNamespaceNewestFirst runs a chain into an empty namespace through
// Run: it is restored newest first, under the unique policy.
func TestRestoreExecutor_Run_EmptyNamespaceNewestFirst(t *testing.T) {
	ctrl := gomock.NewController(t)
	operations, client := expectRestoresInOrder(ctrl, "incr1", "full")
	info := aerospike.NewMockInfoGetter(ctrl)
	info.EXPECT().GetRecordCount(gomock.Any(), "dest-ns", gomock.Any()).Return(uint64(0), nil)
	client.EXPECT().InfoClient().Return(info)

	full, incr := testRestoreRequest(), testRestoreRequest()
	full.BackupDataPath, incr.BackupDataPath = "full", "incr1"

	chain := NewRestoreExecutor(operations).Run(t.Context(), client, slog.Default(), full, incr)
	require.NoError(t, waitChain(t, chain))
}
