package aerospike

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-client-go/v8"
	"github.com/aerospike/backup-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var cluster = &model.AerospikeCluster{
	ClusterLabel: "test",
}

var cluster2 = &model.AerospikeCluster{
	ClusterLabel: "test2",
}

func Test_GetClient(t *testing.T) {
	ctrl := gomock.NewController(t)

	clientFactory := NewMockClientFactory(ctrl)
	mockAsClient := NewMockAerospikeClient(ctrl)
	mockBackupClient := NewMockClient(ctrl)

	infoGetter := NewMockInfoGetter(ctrl)
	infoGetter.EXPECT().GetStatus(gomock.Any()).Return("ok", nil).Times(2)
	mockBackupClient.EXPECT().InfoClient().Return(infoGetter).Times(2)

	clientFactory.EXPECT().NewClientWithPolicyAndHost(gomock.Any(), gomock.Any()).Return(mockAsClient, nil)
	clientFactory.EXPECT().NewBackupClient(gomock.Any(), gomock.Any()).Return(mockBackupClient, nil).Times(2)

	clientManager := NewClientManager(
		clientFactory,
		10*time.Second,
	)

	// First call will create a new client
	client, err := clientManager.GetClient(t.Context(), cluster, nil, nil)
	require.NoError(t, err)
	assert.NotNil(t, client)

	// Second call will reuse the existing client
	client2, err := clientManager.GetClient(t.Context(), cluster, nil, nil)
	require.NoError(t, err)
	assert.NotNil(t, client2)
	assert.Equal(t, client, client2)
}

func Test_GetClientParallel(t *testing.T) {
	ctrl := gomock.NewController(t)

	clientFactory := NewMockClientFactory(ctrl)
	mockAsClient := NewMockAerospikeClient(ctrl)

	mockBackupClient := NewMockClient(ctrl)

	infoGetter := NewMockInfoGetter(ctrl)
	infoGetter.EXPECT().GetStatus(gomock.Any()).Return("ok", nil).Times(2)
	mockBackupClient.EXPECT().InfoClient().Return(infoGetter).Times(2)

	clientFactory.EXPECT().NewClientWithPolicyAndHost(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ *model.AerospikeCluster) (backup.AerospikeClient, error) {
			time.Sleep(100 * time.Millisecond)
			return mockAsClient, nil
		})
	clientFactory.EXPECT().NewBackupClient(gomock.Any(), gomock.Any()).Return(mockBackupClient, nil).Times(2)

	clientManager := NewClientManager(
		clientFactory,
		10*time.Second,
	)

	var client, client2 Client
	var err, err2 error
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		client, err = clientManager.GetClient(t.Context(), cluster, nil, nil)
		wg.Done()
	}()
	go func() {
		client2, err2 = clientManager.GetClient(t.Context(), cluster, nil, nil)
		wg.Done()
	}()
	wg.Wait()
	require.NoError(t, err)
	require.NotNil(t, client)

	require.NoError(t, err2)
	require.NotNil(t, client2)

	assert.Same(t, client, client2)
}

func Test_GetTwoClients(t *testing.T) {
	ctrl := gomock.NewController(t)
	clientFactory := NewMockClientFactory(ctrl)
	mockAsClient := NewMockAerospikeClient(ctrl)
	clientFactory.EXPECT().NewClientWithPolicyAndHost(gomock.Any(), gomock.Any()).Return(mockAsClient, nil).Times(2)

	clientFactory.EXPECT().NewBackupClient(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ backup.AerospikeClient, _ ...backup.ClientOpt) (Client, error) {
			client := NewMockClient(ctrl)

			infoGetter := NewMockInfoGetter(ctrl)
			infoGetter.EXPECT().GetStatus(gomock.Any()).Return("ok", nil)
			client.EXPECT().InfoClient().Return(infoGetter)

			return client, nil
		}).Times(2)

	clientManager := NewClientManager(
		clientFactory,
		10*time.Second,
	)

	client, err := clientManager.GetClient(t.Context(), cluster, nil, nil)
	require.NoError(t, err)
	client2, err := clientManager.GetClient(t.Context(), cluster2, nil, nil)
	require.NoError(t, err)

	assert.NotSame(t, client, client2)
}

func Test_GetClient_UnhealthyConnection(t *testing.T) {
	ctrl := gomock.NewController(t)

	clientFactory := NewMockClientFactory(ctrl)
	mockAsClient := NewMockAerospikeClient(ctrl)

	mockBackupClient := NewMockClient(ctrl)

	clientFactory.EXPECT().NewClientWithPolicyAndHost(gomock.Any(), gomock.Any()).Return(mockAsClient, nil)
	clientFactory.EXPECT().NewBackupClient(gomock.Any(), gomock.Any()).Return(mockBackupClient, nil)

	infoGetter := NewMockInfoGetter(ctrl)
	infoGetter.EXPECT().GetStatus(gomock.Any()).Return("fail", nil)
	mockBackupClient.EXPECT().InfoClient().Return(infoGetter)

	clientManager := NewClientManager(
		clientFactory,
		10*time.Second,
	)

	// Try to get client - should fail due to unhealthy connection
	client, err := clientManager.GetClient(t.Context(), cluster, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "aerospike cluster connection lost")
	assert.Nil(t, client)
}

func Test_CreateClient_Errors(t *testing.T) {
	ctrl := gomock.NewController(t)

	clientFactory := NewMockClientFactory(ctrl)
	aeroCluster := &model.AerospikeCluster{}

	clientFactory.EXPECT().NewClientWithPolicyAndHost(gomock.Any(), aeroCluster).
		Return(nil, errors.New("failed to connect to aerospike"))

	clientManager := NewClientManager(
		clientFactory,
		10*time.Second,
	)

	client, err := clientManager.GetClient(t.Context(), aeroCluster, nil, nil)
	assert.Nil(t, client)
	require.ErrorContains(t, err, "failed to connect to aerospike")
}

func Test_Close(t *testing.T) {
	ctrl := gomock.NewController(t)

	clientFactory := NewMockClientFactory(ctrl)
	mockAsClient := NewMockAerospikeClient(ctrl)
	mockAsClient.EXPECT().Close()

	mockBackupClient := NewMockClient(ctrl)
	mockBackupClient.EXPECT().AerospikeClient().Return(mockAsClient).AnyTimes()

	infoGetter := NewMockInfoGetter(ctrl)
	infoGetter.EXPECT().GetStatus(gomock.Any()).Return("ok", nil)
	mockBackupClient.EXPECT().InfoClient().Return(infoGetter)

	clientFactory.EXPECT().NewClientWithPolicyAndHost(gomock.Any(), gomock.Any()).Return(mockAsClient, nil)
	clientFactory.EXPECT().NewBackupClient(gomock.Any(), gomock.Any()).Return(mockBackupClient, nil)

	clientManager := NewClientManager(
		clientFactory,
		100*time.Millisecond,
	)

	client, err := clientManager.GetClient(t.Context(), cluster, nil, nil)
	require.NoError(t, err)
	assert.NotNil(t, client)
	assertClientExists(t, clientManager, cluster, true)

	clientManager.Close(client)
	time.Sleep(150 * time.Millisecond) // Wait for timer to fire

	assertClientExists(t, clientManager, cluster, false)
	assert.Zero(t, clientManager.clients.Size())
}

func Test_Close_Multiple(t *testing.T) {
	ctrl := gomock.NewController(t)

	clientFactory := NewMockClientFactory(ctrl)
	mockAsClient := NewMockAerospikeClient(ctrl)
	mockAsClient.EXPECT().Close()

	mockBackupClient := NewMockClient(ctrl)
	mockBackupClient.EXPECT().AerospikeClient().Return(mockAsClient).Times(2)

	infoGetter := NewMockInfoGetter(ctrl)
	infoGetter.EXPECT().GetStatus(gomock.Any()).Return("ok", nil).Times(2)
	mockBackupClient.EXPECT().InfoClient().Return(infoGetter).Times(2)

	clientFactory.EXPECT().NewClientWithPolicyAndHost(gomock.Any(), gomock.Any()).Return(mockAsClient, nil)
	clientFactory.EXPECT().NewBackupClient(gomock.Any(), gomock.Any()).Return(mockBackupClient, nil).Times(2)

	clientManager := NewClientManager(
		clientFactory,
		100*time.Millisecond,
	)

	client, err := clientManager.GetClient(t.Context(), cluster, nil, nil)
	require.NoError(t, err)
	assert.NotNil(t, client)
	client, err = clientManager.GetClient(t.Context(), cluster, nil, nil)
	require.NoError(t, err)
	assert.NotNil(t, client)

	clientManager.Close(client)
	assertClientExists(t, clientManager, cluster, true)

	clientManager.Close(client)
	time.Sleep(150 * time.Millisecond) // Wait for timer to fire
	assertClientExists(t, clientManager, cluster, false)
}

func Test_Close_CancelOnReuse(t *testing.T) {
	ctrl := gomock.NewController(t)

	clientFactory := NewMockClientFactory(ctrl)
	mockAsClient := NewMockAerospikeClient(ctrl)

	mockBackupClient := NewMockClient(ctrl)
	mockBackupClient.EXPECT().AerospikeClient().Return(mockAsClient)

	clientFactory.EXPECT().NewClientWithPolicyAndHost(gomock.Any(), gomock.Any()).Return(mockAsClient, nil)
	clientFactory.EXPECT().NewBackupClient(gomock.Any(), gomock.Any()).Return(mockBackupClient, nil).Times(2)

	infoGetter := NewMockInfoGetter(ctrl)
	infoGetter.EXPECT().GetStatus(gomock.Any()).Return("ok", nil).Times(2)
	mockBackupClient.EXPECT().InfoClient().Return(infoGetter).Times(2)

	clientManager := NewClientManager(
		clientFactory,
		100*time.Millisecond,
	)

	client, err := clientManager.GetClient(t.Context(), cluster, nil, nil)
	require.NoError(t, err)
	assert.NotNil(t, client)

	// Schedule closing
	clientManager.Close(client)

	// Reuse the client before it's closed
	time.Sleep(50 * time.Millisecond)
	client2, err := clientManager.GetClient(t.Context(), cluster, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, client, client2)

	// Wait longer than the close delay
	time.Sleep(150 * time.Millisecond)
	assertClientExists(t, clientManager, cluster, true)
}

func Test_Close_NotExisting(t *testing.T) {
	ctrl := gomock.NewController(t)

	clientFactory := NewMockClientFactory(ctrl)
	clientManager := NewClientManager(
		clientFactory,
		10*time.Second,
	)

	aeroClient := NewMockAerospikeClient(ctrl)
	aeroClient.EXPECT().Close()

	client := NewMockClient(ctrl)
	client.EXPECT().AerospikeClient().Return(aeroClient).Times(2)
	aeroClient.EXPECT().Cluster().Return(&aerospike.Cluster{})

	clientManager.Close(client)
}

func assertClientExists(t *testing.T, clientManager *ClientManagerImpl,
	cl *model.AerospikeCluster, shouldExist bool) {
	t.Helper()

	_, exists := clientManager.clients.Load(cl.Hash())
	assert.Equal(t, shouldExist, exists)
}
