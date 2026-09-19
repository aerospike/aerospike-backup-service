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
	wg.Go(func() {
		client, err = clientManager.GetClient(t.Context(), cluster, nil, nil)
	})
	wg.Go(func() {
		client2, err2 = clientManager.GetClient(t.Context(), cluster, nil, nil)
	})
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
	// The dead connection is closed and forgotten, so the next GetClient reconnects.
	mockAsClient.EXPECT().Close()

	manager := NewClientManager(
		clientFactory,
		10*time.Second,
	)

	// Try to get client - should fail due to unhealthy connection
	client, err := manager.GetClient(t.Context(), cluster, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "aerospike cluster connection lost")
	assert.Nil(t, client)

	_, cached := manager.(*clientManager).clients.Load(cluster.Hash())
	assert.False(t, cached, "dead client must not stay cached")
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
	assert.Zero(t, clientCacheSize(clientManager))
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
	client.EXPECT().AerospikeClient().Return(aeroClient)
	aeroClient.EXPECT().Cluster().Return(&aerospike.Cluster{})

	clientManager.Close(client)
}

func assertClientExists(t *testing.T, manager ClientManager,
	cl *model.AerospikeCluster, shouldExist bool) {
	t.Helper()

	_, exists := manager.(*clientManager).clients.Load(cl.Hash())
	assert.Equal(t, shouldExist, exists)
}

func clientCacheSize(manager ClientManager) int {
	return manager.(*clientManager).clients.Size()
}

// Test_Close_DoesNotHoldClientMapWhileWaitingForClientInfo pins the manager's lock order.
//
// dropUnusedClient and scheduleClosing hold clientInfo.mu and then take the clients map write
// lock to Remove. Close must therefore never hold the map while waiting for clientInfo.mu:
// SafeMap.Iterate holds the map lock for the whole callback, so matching the owner inside it
// deadlocks against those two paths, and a pending map writer then blocks every later reader,
// hanging all backups and restores.
func Test_Close_DoesNotHoldClientMapWhileWaitingForClientInfo(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockAsClient := NewMockAerospikeClient(ctrl)

	mockBackupClient := NewMockClient(ctrl)
	mockBackupClient.EXPECT().AerospikeClient().Return(mockAsClient).AnyTimes()

	infoGetter := NewMockInfoGetter(ctrl)
	infoGetter.EXPECT().GetStatus(gomock.Any()).Return("ok", nil)
	mockBackupClient.EXPECT().InfoClient().Return(infoGetter)

	clientFactory := NewMockClientFactory(ctrl)
	clientFactory.EXPECT().NewClientWithPolicyAndHost(gomock.Any(), gomock.Any()).Return(mockAsClient, nil)
	clientFactory.EXPECT().NewBackupClient(gomock.Any(), gomock.Any()).Return(mockBackupClient, nil)

	// A long close delay keeps the idle timer from firing while the test runs.
	manager := NewClientManager(clientFactory, time.Hour)
	cm := manager.(*clientManager)

	client, err := manager.GetClient(t.Context(), cluster, nil, nil)
	require.NoError(t, err)

	info, cached := cm.clients.Load(cluster.Hash())
	require.True(t, cached)

	// Put Close in the position a concurrent dropUnusedClient would: clientInfo.mu is taken.
	info.mu.Lock()

	closeReturned := make(chan struct{})
	go func() {
		defer close(closeReturned)

		manager.Close(client)
	}()

	// Let Close reach the clientInfo.mu it cannot have yet.
	time.Sleep(50 * time.Millisecond)

	mapWritable := make(chan struct{})
	go func() {
		defer close(mapWritable)

		cm.clients.Remove(0)
	}()

	select {
	case <-mapWritable:
	case <-time.After(3 * time.Second):
		info.mu.Unlock()
		t.Fatal("Close holds the clients map while waiting for clientInfo.mu; " +
			"a concurrent dropUnusedClient or scheduleClosing would deadlock against it")
	}

	info.mu.Unlock()

	select {
	case <-closeReturned:
	case <-time.After(3 * time.Second):
		t.Fatal("Close did not return after clientInfo.mu was released")
	}
}
