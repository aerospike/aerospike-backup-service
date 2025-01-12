package aerospike

import (
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	as "github.com/aerospike/aerospike-client-go/v7"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/mocks"
	"github.com/aws/smithy-go/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockClientFactory is a mock implementation of the ClientFactory interface.
type MockClientFactory struct {
	ShouldFail            bool
	IsClusterDisconnected bool
}

var cluster = &model.AerospikeCluster{
	ClusterLabel: ptr.String("test"),
}

func (f *MockClientFactory) NewClientWithPolicyAndHost(_ *as.ClientPolicy, _ ...*as.Host,
) (backup.AerospikeClient, error) {
	if f.ShouldFail {
		return nil, errors.New("failed to connect to aerospike")
	}

	m := &mocks.MockAerospikeClient{}
	m.On("Close").Return()
	m.On("Cluster").Return(&as.Cluster{})
	return m, nil
}

func (f *MockClientFactory) IsClusterHealthy(_ backup.AerospikeClient) bool {
	return !f.IsClusterDisconnected
}

func Test_GetClient(t *testing.T) {
	clientManager := NewClientManager(
		&MockClientFactory{},
		10*time.Second,
	)

	// First call will create a new client
	client, err := clientManager.GetClient(cluster)
	assert.NoError(t, err)
	assert.NotNil(t, client)

	// Second call will reuse the existing client
	client2, err := clientManager.GetClient(cluster)
	assert.NoError(t, err)
	assert.NotNil(t, client2)
	assert.Equal(t, client, client2)
}

func Test_GetClient_UnhealthyConnection(t *testing.T) {
	clientManager := NewClientManager(
		&MockClientFactory{IsClusterDisconnected: true},
		10*time.Second,
	)

	_, _ = clientManager.GetClient(cluster)
	// Try to get client - should fail due to unhealthy connection
	client, err := clientManager.GetClient(cluster)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "aerospike cluster connection lost")
	assert.Nil(t, client)
}

func Test_CreateClient(t *testing.T) {
	clientManager := NewClientManager(
		&MockClientFactory{},
		10*time.Second,
	)

	client, err := clientManager.createClient(&model.AerospikeCluster{})
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func Test_CreateClient_Errors(t *testing.T) {
	mockClientFactory := &MockClientFactory{ShouldFail: true}
	aeroCluster := &model.AerospikeCluster{}

	clientManager := NewClientManager(
		mockClientFactory,
		10*time.Second,
	)

	client, err := clientManager.createClient(aeroCluster)
	assert.Nil(t, client)
	assert.ErrorContains(t, err, "failed to connect to aerospike")
}

func Test_Close(t *testing.T) {
	clientManager := NewClientManager(
		&MockClientFactory{},
		100*time.Millisecond,
	)

	client, err := clientManager.GetClient(cluster)
	assert.NoError(t, err)
	assert.NotNil(t, client)
	assertClientExists(t, clientManager, cluster, true)

	clientManager.Close(client)
	time.Sleep(150 * time.Millisecond) // Wait for timer to fire

	assertClientExists(t, clientManager, cluster, false)
}

func Test_Close_Multiple(t *testing.T) {
	clientManager := NewClientManager(
		&MockClientFactory{},
		100*time.Millisecond,
	)

	client, err := clientManager.GetClient(cluster)
	assert.NoError(t, err)
	assert.NotNil(t, client)
	client, err = clientManager.GetClient(cluster)
	assert.NoError(t, err)
	assert.NotNil(t, client)

	clientManager.Close(client)
	assertClientExists(t, clientManager, cluster, true)

	clientManager.Close(client)
	time.Sleep(150 * time.Millisecond) // Wait for timer to fire
	assertClientExists(t, clientManager, cluster, false)
}

func Test_Close_CancelOnReuse(t *testing.T) {
	clientManager := NewClientManager(
		&MockClientFactory{},
		100*time.Millisecond,
	)

	client, err := clientManager.GetClient(cluster)
	assert.NoError(t, err)
	assert.NotNil(t, client)

	// Schedule closing
	clientManager.Close(client)

	// Reuse the client before it's closed
	time.Sleep(50 * time.Millisecond)
	client2, err := clientManager.GetClient(cluster)
	assert.NoError(t, err)
	assert.Equal(t, client, client2)

	// Wait longer than the close delay
	time.Sleep(150 * time.Millisecond)
	assertClientExists(t, clientManager, cluster, true)
}

func Test_Close_NotExisting(t *testing.T) {
	clientManager := NewClientManager(
		&MockClientFactory{},
		10*time.Second,
	)
	clientManager.SetLogger(slog.Default())
	aeroClient := &mocks.MockAerospikeClient{}
	aeroClient.On("Close").Return()
	aeroClient.On("Cluster").Return(&as.Cluster{})
	client, _ := backup.NewClient(aeroClient)
	clientManager.Close(client)

	aeroClient.AssertExpectations(t)
}

func assertClientExists(t *testing.T, clientManager *ClientManagerImpl,
	cl *model.AerospikeCluster, shouldExist bool) {
	t.Helper()
	clientManager.mu.Lock()
	defer clientManager.mu.Unlock()

	_, exists := clientManager.clients[cl.Hash()]
	assert.Equal(t, shouldExist, exists)
}
