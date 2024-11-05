package service

import (
	"errors"
	"fmt"
	"sync"
	"time"

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

// AerospikeClientFactory defines an interface for creating and checking clients.
type AerospikeClientFactory interface {
	NewClientWithPolicyAndHost(policy *as.ClientPolicy, hosts ...*as.Host) (backup.AerospikeClient, error)
	IsClusterHealthy(client backup.AerospikeClient) bool
}

// DefaultClientFactory is the default implementation of AerospikeClientFactory.
type DefaultClientFactory struct{}

// NewClientWithPolicyAndHost creates a new Aerospike client with the given policy and hosts.
func (f *DefaultClientFactory) NewClientWithPolicyAndHost(
	policy *as.ClientPolicy, hosts ...*as.Host,
) (backup.AerospikeClient, error) {
	return as.NewClientWithPolicyAndHost(policy, hosts...)
}

// IsClusterHealthy checks if the cluster is connected and responding.
func (f *DefaultClientFactory) IsClusterHealthy(client backup.AerospikeClient) bool {
	if client == nil {
		return false
	}

	cluster := client.Cluster()
	if !cluster.IsConnected() {
		return false
	}

	node, err := cluster.GetRandomNode()
	if err != nil {
		return false
	}

	info, err := node.RequestInfo(nil, "status")
	return err == nil && info["status"] == "ok"
}

// ClientManagerImpl implements [ClientManager].
// Is responsible for creating and closing backup clients.
type ClientManagerImpl struct {
	mu            sync.RWMutex
	clients       map[*model.AerospikeCluster]*clientInfo
	clientFactory AerospikeClientFactory
	closeDelay    time.Duration
}

type clientInfo struct {
	client     *backup.Client
	count      int
	closeTimer *time.Timer
}

// NewClientManager creates a new ClientManagerImpl.
func NewClientManager(aerospikeClientFactory AerospikeClientFactory, closeDelay time.Duration) *ClientManagerImpl {
	return &ClientManagerImpl{
		clients:       make(map[*model.AerospikeCluster]*clientInfo),
		clientFactory: aerospikeClientFactory,
		closeDelay:    closeDelay,
	}
}

// GetClient returns a backup client by aerospike cluster name (new or cached).
func (cm *ClientManagerImpl) GetClient(cluster *model.AerospikeCluster) (*backup.Client, error) {
	client, err := cm.getExistingClient(cluster)
	if err != nil {
		return nil, err
	}
	if client != nil {
		return client, nil
	}

	client, err = cm.createClient(cluster)
	if err != nil {
		return nil, fmt.Errorf("cannot create backup client: %w", err)
	}

	return cm.storeClient(cluster, client), nil
}

// getExistingClient tries to get an existing client from the cache.
// Returns nil if client doesn't exist, error is client is not connected.
func (cm *ClientManagerImpl) getExistingClient(cluster *model.AerospikeCluster) (*backup.Client, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if info, exists := cm.clients[cluster]; exists {
		if !cm.clientFactory.IsClusterHealthy(info.client.AerospikeClient()) {
			return nil, errors.New("aerospike cluster connection lost")
		}

		cm.incrementRef(info)
		return info.client, nil
	}

	return nil, nil
}

// storeClient attempts to store the client in the cache.
func (cm *ClientManagerImpl) storeClient(cluster *model.AerospikeCluster, client *backup.Client) *backup.Client {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// If another client was created concurrently,
	// closes the provided client and returns the existing one.
	if info, exists := cm.clients[cluster]; exists {
		client.AerospikeClient().Close()
		cm.incrementRef(info)
		return info.client
	}

	cm.clients[cluster] = &clientInfo{
		client: client,
		count:  1,
	}

	return client
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
			cm.decrementRef(info, id)
			return
		}
	}

	// Close client even if it was not found
	client.AerospikeClient().Close()
}

// incrementRef increases the reference count and cancels any pending close operation.
func (cm *ClientManagerImpl) incrementRef(info *clientInfo) {
	if info.closeTimer != nil {
		info.closeTimer.Stop()
		info.closeTimer = nil
	}
	info.count++
}

// decrementRef decreases the reference count and schedules closing if count reaches zero.
func (cm *ClientManagerImpl) decrementRef(info *clientInfo, cluster *model.AerospikeCluster) {
	info.count--
	if info.count == 0 {
		info.closeTimer = cm.scheduleClosing(cluster)
	}
}

// scheduleClosing schedules client closing after the configured delay.
// Returns a timer that can be used to cancel the scheduled closing if needed.
func (cm *ClientManagerImpl) scheduleClosing(cluster *model.AerospikeCluster) *time.Timer {
	return time.AfterFunc(cm.closeDelay, func() {
		cm.mu.Lock()
		defer cm.mu.Unlock()

		// Check if the client still exists and count is still 0
		if info, exists := cm.clients[cluster]; exists && info.count == 0 {
			info.client.AerospikeClient().Close()
			delete(cm.clients, cluster)
		}
	})
}
