package backupexecutor

import (
	"errors"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCloseOnWaitBackupHandler(t *testing.T) {
	ctrl := gomock.NewController(t)

	inner := NewMockBackupHandler(ctrl)
	client := aerospike.NewMockClient(ctrl)
	clientManager := aerospike.NewMockClientManager(ctrl)

	stats := models.NewBackupStats()
	metrics := models.NewMetrics(1, 2, 42, 10)

	inner.EXPECT().Wait(gomock.Any()).Return(nil)
	inner.EXPECT().GetStats().Return(stats)
	inner.EXPECT().GetMetrics().Return(metrics)
	clientManager.EXPECT().Close(client).Times(1)

	handler := &closeOnWaitBackupHandler{
		inner:         inner,
		client:        client,
		clientManager: clientManager,
	}

	require.NoError(t, handler.Wait(t.Context()))
	assert.Equal(t, stats, handler.GetStats())
	assert.Equal(t, metrics, handler.GetMetrics())

	// Close is invoked exactly once even if Wait is called again.
	inner.EXPECT().Wait(gomock.Any()).Return(nil)
	require.NoError(t, handler.Wait(t.Context()))
}

func TestCloseOnWaitBackupHandler_WaitErrorStillClosesClient(t *testing.T) {
	ctrl := gomock.NewController(t)

	inner := NewMockBackupHandler(ctrl)
	client := aerospike.NewMockClient(ctrl)
	clientManager := aerospike.NewMockClientManager(ctrl)

	waitErr := errors.New("backup wait failed")
	inner.EXPECT().Wait(gomock.Any()).Return(waitErr)
	clientManager.EXPECT().Close(client).Times(1)

	handler := &closeOnWaitBackupHandler{
		inner:         inner,
		client:        client,
		clientManager: clientManager,
	}

	require.ErrorIs(t, handler.Wait(t.Context()), waitErr)
}
