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

	assert.Equal(t, client, client2, "parallel callers share one connection")
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

	assert.NotEqual(t, client, client2)
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

// Closing one cluster's client must neither wait for another cluster's health check nor
// deadlock against the drop that follows a failed one.
func Test_Close_DuringFailingHealthCheck(t *testing.T) {
	ctrl := gomock.NewController(t)
	clientFactory := NewMockClientFactory(ctrl)

	healthyAsClient := NewMockAerospikeClient(ctrl)
	healthyInfo := NewMockInfoGetter(ctrl)
	healthyInfo.EXPECT().GetStatus(gomock.Any()).Return("ok", nil)
	healthyBackupClient := NewMockClient(ctrl)
	healthyBackupClient.EXPECT().InfoClient().Return(healthyInfo)

	deadAsClient := NewMockAerospikeClient(ctrl)
	deadAsClient.EXPECT().Close()
	healthCheckStarted := make(chan struct{})
	healthCheckDone := make(chan struct{})
	deadInfo := NewMockInfoGetter(ctrl)
	deadInfo.EXPECT().GetStatus(gomock.Any()).DoAndReturn(func(context.Context) (string, error) {
		close(healthCheckStarted)
		<-healthCheckDone
		return "fail", nil
	})
	deadBackupClient := NewMockClient(ctrl)
	deadBackupClient.EXPECT().InfoClient().Return(deadInfo)

	clientFactory.EXPECT().NewClientWithPolicyAndHost(gomock.Any(), cluster).Return(healthyAsClient, nil)
	clientFactory.EXPECT().NewClientWithPolicyAndHost(gomock.Any(), cluster2).Return(deadAsClient, nil)
	// The healthy cluster connects first, the dead one second.
	clientFactory.EXPECT().NewBackupClient(gomock.Any(), gomock.Any()).Return(healthyBackupClient, nil)
	clientFactory.EXPECT().NewBackupClient(gomock.Any(), gomock.Any()).Return(deadBackupClient, nil)

	manager := NewClientManager(clientFactory, 10*time.Second)

	healthyClient, err := manager.GetClient(t.Context(), cluster, nil, nil)
	require.NoError(t, err)

	getDeadDone := make(chan struct{})
	go func() {
		defer close(getDeadDone)
		_, err := manager.GetClient(t.Context(), cluster2, nil, nil)
		assert.Error(t, err)
	}()
	<-healthCheckStarted

	closeDone := make(chan struct{})
	go func() {
		defer close(closeDone)
		manager.Close(healthyClient)
	}()
	awaitDone(t, closeDone, "Close of a healthy cluster's client during another cluster's health check")

	close(healthCheckDone)
	awaitDone(t, getDeadDone, "GetClient after a failed health check")

	assertClientExists(t, manager, cluster, true)
	assertClientExists(t, manager, cluster2, false)
}

// A caller waiting for an entry whose health check fails under another caller must not revive
// the dropped entry: it gets a fresh one that is in the map, so its own Close is counted.
func Test_GetClient_WaitersGetFreshEntryAfterDrop(t *testing.T) {
	ctrl := gomock.NewController(t)
	clientFactory := NewMockClientFactory(ctrl)

	deadAsClient := NewMockAerospikeClient(ctrl)
	deadAsClient.EXPECT().Close()
	healthCheckStarted := make(chan struct{})
	healthCheckDone := make(chan struct{})
	deadInfo := NewMockInfoGetter(ctrl)
	deadInfo.EXPECT().GetStatus(gomock.Any()).DoAndReturn(func(context.Context) (string, error) {
		close(healthCheckStarted)
		<-healthCheckDone
		return "fail", nil
	})
	deadBackupClient := NewMockClient(ctrl)
	deadBackupClient.EXPECT().InfoClient().Return(deadInfo)

	liveAsClient := NewMockAerospikeClient(ctrl)
	liveInfo := NewMockInfoGetter(ctrl)
	liveInfo.EXPECT().GetStatus(gomock.Any()).Return("ok", nil)
	liveBackupClient := NewMockClient(ctrl)
	liveBackupClient.EXPECT().InfoClient().Return(liveInfo)

	// The first caller connects and fails; the waiter connects anew.
	clientFactory.EXPECT().NewClientWithPolicyAndHost(gomock.Any(), cluster).Return(deadAsClient, nil)
	clientFactory.EXPECT().NewClientWithPolicyAndHost(gomock.Any(), cluster).Return(liveAsClient, nil)
	clientFactory.EXPECT().NewBackupClient(gomock.Any(), gomock.Any()).Return(deadBackupClient, nil)
	clientFactory.EXPECT().NewBackupClient(gomock.Any(), gomock.Any()).Return(liveBackupClient, nil)

	manager := NewClientManager(clientFactory, 10*time.Second)

	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		_, err := manager.GetClient(t.Context(), cluster, nil, nil)
		assert.Error(t, err)
	}()
	<-healthCheckStarted

	waiterClient := make(chan Client, 1)
	go func() {
		client, err := manager.GetClient(t.Context(), cluster, nil, nil)
		assert.NoError(t, err)
		waiterClient <- client
	}()
	time.Sleep(50 * time.Millisecond) // let the waiter block on the entry
	close(healthCheckDone)
	awaitDone(t, firstDone, "GetClient after a failed health check")

	var client Client
	select {
	case client = <-waiterClient:
	case <-time.After(3 * time.Second):
		t.Fatal("waiting GetClient did not finish")
	}

	entry, cached := manager.(*clientManager).clients.Load(cluster.Hash())
	require.True(t, cached, "the waiter must have stored a fresh entry")
	assert.Same(t, entry, client.(managedClient).info)
}

func Test_ClientInfo_CloseIfUnused(t *testing.T) {
	tests := []struct {
		name          string
		count         int
		closed        bool
		wantClosed    bool
		wantConnClose bool
	}{
		{name: "unused connection is closed", wantClosed: true, wantConnClose: true},
		{name: "held connection stays open", count: 1},
		{name: "closed entry stays closed", closed: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			asClient := NewMockAerospikeClient(ctrl)
			if tt.wantConnClose {
				asClient.EXPECT().Close()
			}

			info := newClientInfo(cluster.Hash(), cluster)
			info.aeroClient = asClient
			info.count = tt.count
			info.closed = tt.closed

			assert.Equal(t, tt.wantClosed, info.closeIfUnused())
		})
	}
}

func Test_ClientInfo_ClosedEntryIsNotReused(t *testing.T) {
	info := newClientInfo(cluster.Hash(), cluster)

	require.True(t, info.closeIfUnused())
	assert.False(t, info.closeIfUnused(), "an entry closes once")

	_, err := info.acquire(t.Context(), NewMockClientFactory(gomock.NewController(t)), nil, nil)
	require.ErrorIs(t, err, errClientInfoClosed)
}

func awaitDone(t *testing.T, done <-chan struct{}, what string) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("%s did not finish", what)
	}
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
