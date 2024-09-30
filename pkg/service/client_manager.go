package service

import (
	"fmt"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	as "github.com/aerospike/aerospike-client-go/v7"
	"github.com/aerospike/backup-go"
	"golang.org/x/sync/semaphore"
)

// ClientManager is responsible for creating and closing backup clients.
type ClientManager interface {
	// GetClient returns a backup client by aerospike cluster name (new or cached).
	GetClient(*model.AerospikeCluster) (*backup.Client, error)
	// Close ensures that the specified backup client is closed.
	Close(*backup.Client)
}

// AerospikeClientFactory defines an interface for creating new clients.
type AerospikeClientFactory interface {
	NewClientWithPolicyAndHost(policy *as.ClientPolicy, hosts ...*as.Host) (backup.AerospikeClient, error)
}

// DefaultClientFactory is the default implementation of AerospikeClientFactory.
type DefaultClientFactory struct{}

// NewClientWithPolicyAndHost creates a new Aerospike client with the given policy and hosts.
func (f *DefaultClientFactory) NewClientWithPolicyAndHost(
	policy *as.ClientPolicy, hosts ...*as.Host,
) (backup.AerospikeClient, error) {
	return as.NewClientWithPolicyAndHost(policy, hosts...)
}

// ClientManagerImpl implements [ClientManager].
// Is responsible for creating and closing backup clients.
type ClientManagerImpl struct {
	mu            sync.Mutex
	clients       map[*model.AerospikeCluster]*clientInfo
	clientFactory AerospikeClientFactory
}

type clientInfo struct {
	client *backup.Client
	count  int
}

// NewClientManager creates a new ClientManagerImpl.
func NewClientManager(aerospikeClientFactory AerospikeClientFactory) *ClientManagerImpl {
	return &ClientManagerImpl{
		clients:       make(map[*model.AerospikeCluster]*clientInfo),
		clientFactory: aerospikeClientFactory,
	}
}

// GetClient returns a backup client by aerospike cluster name (new or cached).
func (cm *ClientManagerImpl) GetClient(cluster *model.AerospikeCluster) (*backup.Client, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if info, exists := cm.clients[cluster]; exists {
		info.count++
		return info.client, nil
	}

	client, err := cm.createClient(cluster)
	if err != nil {
		return nil, fmt.Errorf("cannot create backup client: %w", err)
	}

	cm.clients[cluster] = &clientInfo{
		client: client,
		count:  1,
	}

	return client, nil
}

// createClient creates a new backup client given the aerospike cluster configuration.
func (cm *ClientManagerImpl) createClient(cluster *model.AerospikeCluster) (*backup.Client, error) {
	aeroClient, err := cm.clientFactory.NewClientWithPolicyAndHost(cluster.ASClientPolicy(),
		cluster.ASClientHosts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to aerospike cluster, %w", err)
	}

	var options []backup.ClientOpt
	if cluster.MaxParallelScans != nil {
		options = append(options, backup.WithScanLimiter(
			semaphore.NewWeighted(int64(*cluster.MaxParallelScans))))
	}
	if cluster.ClusterLabel != nil {
		options = append(options, backup.WithID(*cluster.ClusterLabel))
	}

	return backup.NewClient(aeroClient, options...)
}

// Close ensures that the specified backup client is closed.
func (cm *ClientManagerImpl) Close(client *backup.Client) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for id, info := range cm.clients {
		if info.client == client {
			info.count--
			if info.count == 0 {
				info.client.AerospikeClient().Close()
				delete(cm.clients, id)
			}
			return
		}
	}

	// close client even it was not found
	client.AerospikeClient().Close()
}
