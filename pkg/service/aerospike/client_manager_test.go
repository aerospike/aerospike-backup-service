package aerospike

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/mocks"
	"github.com/aws/smithy-go/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var cluster = &model.AerospikeCluster{
	ClusterLabel: ptr.String("test"),
}

var cluster2 = &model.AerospikeCluster{
	ClusterLabel: ptr.String("test2"),
}

func Test_GetClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientFactory := NewMockClientFactory(ctrl)
	mockAsClient := mocks.NewMockAerospikeClient(t)
	mockBackupClient := NewMockClient(ctrl)

	infoGetter := mocks.NewMockInfoGetter(t)
	infoGetter.EXPECT().GetStatus(mock.Anything).Return("ok", nil)
	mockBackupClient.EXPECT().InfoClient().Return(infoGetter)

	clientFactory.EXPECT().NewClientWithPolicyAndHost(gomock.Any()).Return(mockAsClient, nil)
	clientFactory.EXPECT().NewBackupClient(gomock.Any(), gomock.Any()).Return(mockBackupClient, nil)

	clientManager := NewClientManager(
		clientFactory,
		10*time.Second,
	)

	// First call will create a new client
	client, err := clientManager.GetClient(context.Background(), cluster, "1")
	assert.NoError(t, err)
	assert.NotNil(t, client)

	// Second call will reuse the existing client
	client2, err := clientManager.GetClient(context.Background(), cluster, "2")
	assert.NoError(t, err)
	assert.NotNil(t, client2)
	assert.Equal(t, client, client2)
}

func Test_GetClientParallel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientFactory := NewMockClientFactory(ctrl)
	mockAsClient := mocks.NewMockAerospikeClient(t)

	mockBackupClient := NewMockClient(ctrl)

	infoGetter := mocks.NewMockInfoGetter(t)
	infoGetter.EXPECT().GetStatus(mock.Anything).Return("ok", nil)
	mockBackupClient.EXPECT().InfoClient().Return(infoGetter)

	clientFactory.EXPECT().NewClientWithPolicyAndHost(gomock.Any()).
		DoAndReturn(func(_ *model.AerospikeCluster) (backup.AerospikeClient, error) {
			time.Sleep(100 * time.Millisecond)
			return mockAsClient, nil
		})
	clientFactory.EXPECT().NewBackupClient(gomock.Any(), gomock.Any()).Return(mockBackupClient, nil)

	clientManager := NewClientManager(
		clientFactory,
		10*time.Second,
	)

	var client, client2 Client
	var err, err2 error
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		client, err = clientManager.GetClient(context.Background(), cluster, "1")
		wg.Done()
	}()
	go func() {
		client2, err2 = clientManager.GetClient(context.Background(), cluster, "2")
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
	defer ctrl.Finish()
	clientFactory := NewMockClientFactory(ctrl)
	mockAsClient := mocks.NewMockAerospikeClient(t)
	clientFactory.EXPECT().NewClientWithPolicyAndHost(gomock.Any()).Return(mockAsClient, nil).Times(2)

	clientFactory.EXPECT().NewBackupClient(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ backup.AerospikeClient, _ ...backup.ClientOpt) (Client, error) {
			return NewMockClient(ctrl), nil
		}).Times(2)

	clientManager := NewClientManager(
		clientFactory,
		10*time.Second,
	)

	client, err := clientManager.GetClient(context.Background(), cluster, "1")
	require.NoError(t, err)
	client2, err := clientManager.GetClient(context.Background(), cluster2, "2")
	require.NoError(t, err)

	assert.NotSame(t, client, client2)
}

func Test_GetClient_UnhealthyConnection(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientFactory := NewMockClientFactory(ctrl)
	mockAsClient := mocks.NewMockAerospikeClient(t)

	mockBackupClient := NewMockClient(ctrl)

	clientFactory.EXPECT().NewClientWithPolicyAndHost(gomock.Any()).Return(mockAsClient, nil)
	clientFactory.EXPECT().NewBackupClient(gomock.Any(), gomock.Any()).Return(mockBackupClient, nil)

	infoGetter := mocks.NewMockInfoGetter(t)
	infoGetter.EXPECT().GetStatus(mock.Anything).Return("fail", nil)
	mockBackupClient.EXPECT().InfoClient().Return(infoGetter)

	clientManager := NewClientManager(
		clientFactory,
		10*time.Second,
	)

	_, _ = clientManager.GetClient(context.Background(), cluster, "1")
	// Try to get client - should fail due to unhealthy connection
	client, err := clientManager.GetClient(context.Background(), cluster, "2")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "aerospike cluster connection lost")
	assert.Nil(t, client)
}

func Test_CreateClient_Errors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientFactory := NewMockClientFactory(ctrl)
	aeroCluster := &model.AerospikeCluster{}

	clientFactory.EXPECT().NewClientWithPolicyAndHost(aeroCluster).
		Return(nil, errors.New("failed to connect to aerospike"))

	clientManager := NewClientManager(
		clientFactory,
		10*time.Second,
	)

	client, err := clientManager.GetClient(context.Background(), aeroCluster, "")
	assert.Nil(t, client)
	assert.ErrorContains(t, err, "failed to connect to aerospike")
}

func Test_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientFactory := NewMockClientFactory(ctrl)
	mockAsClient := mocks.NewMockAerospikeClient(t)
	mockAsClient.EXPECT().Close()

	mockBackupClient := NewMockClient(ctrl)
	mockBackupClient.EXPECT().AerospikeClient().Return(mockAsClient)

	clientFactory.EXPECT().NewClientWithPolicyAndHost(gomock.Any()).Return(mockAsClient, nil)
	clientFactory.EXPECT().NewBackupClient(gomock.Any(), gomock.Any()).Return(mockBackupClient, nil)

	clientManager := NewClientManager(
		clientFactory,
		100*time.Millisecond,
	)

	client, err := clientManager.GetClient(context.Background(), cluster, "1")
	assert.NoError(t, err)
	assert.NotNil(t, client)
	assertClientExists(t, clientManager, cluster, true)

	clientManager.Close(client)
	time.Sleep(150 * time.Millisecond) // Wait for timer to fire

	assertClientExists(t, clientManager, cluster, false)
	require.Empty(t, clientManager.clients)
}

func Test_Close_Multiple(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientFactory := NewMockClientFactory(ctrl)
	mockAsClient := mocks.NewMockAerospikeClient(t)
	mockAsClient.EXPECT().Close()

	mockBackupClient := NewMockClient(ctrl)
	mockBackupClient.EXPECT().AerospikeClient().Return(mockAsClient)

	infoGetter := mocks.NewMockInfoGetter(t)
	infoGetter.EXPECT().GetStatus(mock.Anything).Return("ok", nil)
	mockBackupClient.EXPECT().InfoClient().Return(infoGetter)

	clientFactory.EXPECT().NewClientWithPolicyAndHost(gomock.Any()).Return(mockAsClient, nil)
	clientFactory.EXPECT().NewBackupClient(gomock.Any(), gomock.Any()).Return(mockBackupClient, nil)

	clientManager := NewClientManager(
		clientFactory,
		100*time.Millisecond,
	)

	client, err := clientManager.GetClient(context.Background(), cluster, "1")
	assert.NoError(t, err)
	assert.NotNil(t, client)
	client, err = clientManager.GetClient(context.Background(), cluster, "2")
	assert.NoError(t, err)
	assert.NotNil(t, client)

	clientManager.Close(client)
	assertClientExists(t, clientManager, cluster, true)

	clientManager.Close(client)
	time.Sleep(150 * time.Millisecond) // Wait for timer to fire
	assertClientExists(t, clientManager, cluster, false)
}

func Test_Close_CancelOnReuse(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientFactory := NewMockClientFactory(ctrl)
	mockAsClient := mocks.NewMockAerospikeClient(t)

	mockBackupClient := NewMockClient(ctrl)

	clientFactory.EXPECT().NewClientWithPolicyAndHost(gomock.Any()).Return(mockAsClient, nil)
	clientFactory.EXPECT().NewBackupClient(gomock.Any(), gomock.Any()).Return(mockBackupClient, nil)

	infoGetter := mocks.NewMockInfoGetter(t)
	infoGetter.EXPECT().GetStatus(mock.Anything).Return("ok", nil)
	mockBackupClient.EXPECT().InfoClient().Return(infoGetter)

	clientManager := NewClientManager(
		clientFactory,
		100*time.Millisecond,
	)

	client, err := clientManager.GetClient(context.Background(), cluster, "1")
	assert.NoError(t, err)
	assert.NotNil(t, client)

	// Schedule closing
	clientManager.Close(client)

	// Reuse the client before it's closed
	time.Sleep(50 * time.Millisecond)
	client2, err := clientManager.GetClient(context.Background(), cluster, "2")
	assert.NoError(t, err)
	assert.Equal(t, client, client2)

	// Wait longer than the close delay
	time.Sleep(150 * time.Millisecond)
	assertClientExists(t, clientManager, cluster, true)
}

func Test_Close_NotExisting(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientFactory := NewMockClientFactory(ctrl)
	clientManager := NewClientManager(
		clientFactory,
		10*time.Second,
	)

	aeroClient := mocks.NewMockAerospikeClient(t)
	aeroClient.EXPECT().Close()

	client := NewMockClient(ctrl)
	client.EXPECT().AerospikeClient().Return(aeroClient)

	clientManager.Close(client)

	aeroClient.AssertExpectations(t)
}

func assertClientExists(t *testing.T, clientManager *ClientManagerImpl,
	cl *model.AerospikeCluster, shouldExist bool) {
	t.Helper()

	_, exists := clientManager.clients.Load(cl.Hash())
	assert.Equal(t, shouldExist, exists)
}
