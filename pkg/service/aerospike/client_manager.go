package aerospike

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	as "github.com/aerospike/aerospike-client-go/v8"
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

// ClientFactory defines an interface for creating and checking clients.
type ClientFactory interface {
	NewClientWithPolicyAndHost(*model.AerospikeCluster) (backup.AerospikeClient, error)
	IsClusterHealthy(client Cluster) bool
}

type Cluster interface {
	Cluster() *as.Cluster
}

// ClientManagerImpl implements [ClientManager].
// Is responsible for creating and closing backup clients.
type ClientManagerImpl struct {
	mu sync.Mutex

	clients       map[string]*clientInfo
	clientFactory ClientFactory
	closeDelay    time.Duration

	logger *slog.Logger
	locks  sync.Map
}

type clientInfo struct {
	client     *backup.Client
	count      int
	closeTimer *time.Timer
}

// NewClientManager creates a new ClientManagerImpl.
func NewClientManager(aerospikeClientFactory ClientFactory, closeDelay time.Duration) *ClientManagerImpl {
	return &ClientManagerImpl{
		clients:       make(map[string]*clientInfo),
		clientFactory: aerospikeClientFactory,
		closeDelay:    closeDelay,
	}
}

// SetLogger sets the logger for the ClientManagerImpl.
// Needs to be set after instantiation due to the initialization order in main.
func (cm *ClientManagerImpl) SetLogger(logger *slog.Logger) {
	cm.logger = logger
}

// GetClient returns a backup client by aerospike cluster name (new or cached).
func (cm *ClientManagerImpl) GetClient(cluster *model.AerospikeCluster) (*backup.Client, error) {
	if cluster == nil {
		return nil, errors.New("cluster is nil")
	}

	clusterKey := cluster.Hash()

	// Try getting an existing client under the global lock.
	cm.mu.Lock()
	info, exists := cm.clients[clusterKey]
	cm.mu.Unlock()

	// Get or create mutex for this client.
	m := cm.mutexForCluster(clusterKey)
	m.Lock()
	defer m.Unlock()

	if exists {
		return cm.checkHealthAndIncrement(info)
	}

	// Check again since another goroutine might have created the client while we were waiting on lock.
	cm.mu.Lock()
	info, exists = cm.clients[clusterKey]
	cm.mu.Unlock()

	if exists {
		return cm.checkHealthAndIncrement(info)
	}

	client, err := cm.createClient(cluster)
	if err != nil {
		return nil, fmt.Errorf("cannot create backup client: %w", err)
	}

	// Store the newly created client (under the global lock).
	cm.storeClient(clusterKey, client)

	return client, nil
}

func (cm *ClientManagerImpl) checkHealthAndIncrement(info *clientInfo) (*backup.Client, error) {
	if !cm.clientFactory.IsClusterHealthy(info.client.AerospikeClient()) {
		return nil, errors.New("aerospike cluster connection lost")
	}

	cm.incrementRef(info)
	return info.client, nil
}

func (cm *ClientManagerImpl) mutexForCluster(clusterKey string) *sync.Mutex {
	stored, _ := cm.locks.LoadOrStore(clusterKey, &sync.Mutex{})
	return stored.(*sync.Mutex)
}

func (cm *ClientManagerImpl) storeClient(clusterKey string, client *backup.Client) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.clients[clusterKey] = &clientInfo{
		client: client,
		count:  1,
	}

	if cm.logger != nil {
		cm.logger.Info("Created new backup client",
			slog.Int("len", len(cm.clients)),
			slog.String("key", clusterKey))
	}
}

// createClient creates a new backup client given the aerospike cluster configuration.
func (cm *ClientManagerImpl) createClient(cluster *model.AerospikeCluster) (*backup.Client, error) {
	aeroClient, err := cm.clientFactory.NewClientWithPolicyAndHost(cluster)
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

	if cm.logger != nil {
		cm.logger.Info("Aerospike client not found and closed",
			slog.Any("hosts", client.AerospikeClient().Cluster().GetSeeds()))
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
func (cm *ClientManagerImpl) decrementRef(info *clientInfo, clusterKey string) {
	info.count--
	if info.count == 0 {
		info.closeTimer = cm.scheduleClosing(clusterKey)
	}
}

// scheduleClosing schedules client closing after the configured delay.
// Returns a timer that can be used to cancel the scheduled closing if needed.
func (cm *ClientManagerImpl) scheduleClosing(clusterKey string) *time.Timer {
	return time.AfterFunc(cm.closeDelay, func() {
		cm.mu.Lock()
		// Check if the client still exists and count is still 0
		if info, exists := cm.clients[clusterKey]; exists && info.count == 0 {
			client := info.client.AerospikeClient()
			delete(cm.clients, clusterKey)
			cm.locks.Delete(clusterKey)
			cm.mu.Unlock()

			client.Close()
			if cm.logger != nil {
				cm.logger.Info("Aerospike client closed",
					slog.Any("hosts", client.Cluster().GetSeeds()),
					slog.Int("len", len(cm.clients)),
					slog.Any("id", clusterKey),
				)
			}

			return
		}
		cm.mu.Unlock()
	})
}
