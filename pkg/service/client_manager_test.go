package service

import (
	"errors"
	"github.com/aerospike/aerospike-backup-service/pkg/model"
	"github.com/aws/smithy-go/ptr"
	"testing"

	as "github.com/aerospike/aerospike-client-go/v7"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/mocks"
	"github.com/stretchr/testify/assert"
)

// MockClientFactory is a mock implementation of the AerospikeClientFactory interface.
type MockClientFactory struct {
	ShouldFail bool
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
	return m, nil
}

func Test_GetClient(t *testing.T) {
	clientManager := NewClientManager(
		map[string]*model.AerospikeCluster{"testCluster": {}},
		&MockClientFactory{},
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

func Test_CreateClient(t *testing.T) {
	clientManager := NewClientManager(
		map[string]*model.AerospikeCluster{},
		&MockClientFactory{},
	)

	client, err := clientManager.CreateClient(&model.AerospikeCluster{})
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func Test_CreateClient_Errors(t *testing.T) {
	mockClientFactory := &MockClientFactory{ShouldFail: true}
	aeroCluster := &model.AerospikeCluster{}

	clientManager := NewClientManager(
		map[string]*model.AerospikeCluster{},
		mockClientFactory,
	)

	client, err := clientManager.CreateClient(aeroCluster)
	assert.Nil(t, client)
	assert.ErrorContains(t, err, "failed to connect to aerospike")
}

func Test_Close(t *testing.T) {
	clientManager := NewClientManager(
		map[string]*model.AerospikeCluster{"testCluster": {}},
		&MockClientFactory{},
	)

	client, err := clientManager.GetClient(cluster)
	assert.NoError(t, err)
	assert.NotNil(t, client)

	clientManager.Close(client)

	// Verify that client is removed from clients map
	_, exists := clientManager.clients[cluster]
	assert.False(t, exists)
}

func Test_Close_Multiple(t *testing.T) {
	clientManager := NewClientManager(
		map[string]*model.AerospikeCluster{"testCluster": {}},
		&MockClientFactory{},
	)

	client, err := clientManager.GetClient(cluster)
	assert.NoError(t, err)
	assert.NotNil(t, client)
	client, err = clientManager.GetClient(cluster)
	assert.NoError(t, err)
	assert.NotNil(t, client)

	clientManager.Close(client)

	_, exists := clientManager.clients[cluster]
	assert.True(t, exists)

	clientManager.Close(client)

	_, exists = clientManager.clients[cluster]
	assert.False(t, exists)
}

func Test_Close_NotExisting(t *testing.T) {
	clientManager := NewClientManager(
		map[string]*model.AerospikeCluster{},
		&MockClientFactory{},
	)
	aeroClient := &mocks.MockAerospikeClient{}
	aeroClient.On("Close").Return()
	client, _ := backup.NewClient(aeroClient)
	clientManager.Close(client)

	aeroClient.AssertExpectations(t)
}
