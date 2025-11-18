package aerospike

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/aerospike/backup-go"
	"golang.org/x/sync/semaphore"
)

// ClientManager is responsible for creating and closing backup clients.
type ClientManager interface {
	// GetClient returns a backup client by aerospike cluster name (new or cached).
	GetClient(ctx context.Context, cluster *model.AerospikeCluster, label string) (Client, error)
	// Close ensures that the specified backup client is released (ref count decremented).
	Close(Client)
}

type Cluster interface {
	// Cluster exposes the cluster object to the user
	Cluster() *as.Cluster
}

// ClientManagerImpl implements [ClientManager].
type ClientManagerImpl struct {
	// clients holds the state for each cluster.
	clients       *collections.SafeMap[string, *clientInfo]
	clientFactory ClientFactory
	closeDelay    time.Duration
}

var _ ClientManager = (*ClientManagerImpl)(nil)

type clientInfo struct {
	// mu protects the fields below (count, closeTimer, aeroClient)
	mu          sync.Mutex
	aeroClient  backup.AerospikeClient
	scanLimiter *semaphore.Weighted

	count      int
	closeTimer *time.Timer
}

// NewClientManager creates a new ClientManagerImpl.
func NewClientManager(aerospikeClientFactory ClientFactory, closeDelay time.Duration) *ClientManagerImpl {
	return &ClientManagerImpl{
		clients:       collections.NewSafeMap[string, *clientInfo](),
		clientFactory: aerospikeClientFactory,
		closeDelay:    closeDelay,
	}
}

// GetClient returns a backup client by aerospike cluster name (new or cached).
func (cm *ClientManagerImpl) GetClient(
	ctx context.Context,
	cluster *model.AerospikeCluster,
	label string,
) (Client, error) {
	if cluster == nil {
		return nil, errors.New("cluster is nil")
	}

	clusterKey := cluster.Hash()

	// Get or create the info struct.
	// Note: If created, info.aeroClient is still nil. We handle initialization under the lock below.
	info := cm.clients.LoadOrStore(clusterKey, newInfo(cluster))

	// We must lock to ensure atomic initialization and reference counting.
	info.mu.Lock()
	defer info.mu.Unlock()

	// 1. Initialize connection if needed
	if info.aeroClient == nil {
		aeroClient, err := cm.clientFactory.NewClientWithPolicyAndHost(cluster)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to aerospike cluster: %w", err)
		}
		info.aeroClient = aeroClient
	}

	client, err := cm.createBackupClient(info, label)
	if err != nil {
		return nil, err
	}

	// 2. Check health
	// We have a valid aeroClient, but is it connected?
	status, err := client.InfoClient().GetStatus(ctx)
	if err != nil {
		return nil, err
	}

	if status != "ok" {
		// Connection is dead.
		// The caller will have to retry (which will trigger a new connection).
		return nil, fmt.Errorf("aerospike cluster is not healthy: %s", status)
	}

	// 3. Increment Reference
	if info.closeTimer != nil {
		info.closeTimer.Stop()
		info.closeTimer = nil
	}
	info.count++

	// 4. Create the wrapper for this specific request
	return client, nil
}

func newInfo(cluster *model.AerospikeCluster) *clientInfo {
	value := &clientInfo{}
	if cluster.MaxParallelScans != nil {
		value.scanLimiter = semaphore.NewWeighted(int64(*cluster.MaxParallelScans))
	}
	return value
}

func (cm *ClientManagerImpl) createBackupClient(info *clientInfo, label string) (Client, error) {
	var options []backup.ClientOpt
	options = append(options, backup.WithScanLimiter(info.scanLimiter))
	if len(label) > 0 {
		options = append(options, backup.WithLogger(slog.With(slog.String("label", label))))
	}

	return cm.clientFactory.NewBackupClient(info.aeroClient, options...)
}

// Close ensures that the specified backup client is released.
func (cm *ClientManagerImpl) Close(client Client) {
	var targetInfo *clientInfo
	var targetKey string

	// We need to find which info struct owns this client.
	// Since Client interface wraps the underlying AerospikeClient, we compare pointers.
	found := false
	cm.clients.Iterate(func(key string, info *clientInfo) {
		if found {
			return // Optimization: stop if already found
		}

		// We must lock to read info.aeroClient safely,
		// just in case it's being modified (though unlikely after init).
		info.mu.Lock()
		if info.aeroClient == client.AerospikeClient() {
			targetInfo = info
			targetKey = key
			found = true
		}
		info.mu.Unlock()
	})

	if found {
		cm.decrementRef(targetInfo, targetKey)
	} else {
		// Logic fix: If it's not in our cache, we must close it immediately
		// because we aren't managing its lifecycle.
		client.AerospikeClient().Close()
		slog.Info("Closed Aerospike client not managed by the cache",
			slog.Any("hosts", client.AerospikeClient().Cluster().GetSeeds()))
	}
}

// decrementRef decreases the reference count and schedules closing if count reaches zero.
func (cm *ClientManagerImpl) decrementRef(info *clientInfo, clusterKey string) {
	info.mu.Lock()
	defer info.mu.Unlock()

	info.count--
	if info.count <= 0 {
		// Sanity check to prevent negative counts
		info.count = 0

		// If a timer is already running (edge case), stop it first
		if info.closeTimer != nil {
			info.closeTimer.Stop()
		}
		info.closeTimer = cm.scheduleClosing(clusterKey)
	}
}

// scheduleClosing schedules client closing after the configured delay.
func (cm *ClientManagerImpl) scheduleClosing(clusterKey string) *time.Timer {
	return time.AfterFunc(cm.closeDelay, func() {
		// Timer callback runs in its own goroutine.

		// 1. Retrieve the info struct
		info, exists := cm.clients.Load(clusterKey)
		if !exists {
			return
		}

		// 2. Lock to check state safely
		info.mu.Lock()
		defer info.mu.Unlock()

		// 3. Verify condition: Is count still 0?
		// Someone might have called GetClient() in the milliseconds before this lock was acquired.
		if info.count == 0 {
			// Remove from map to prevent new users from picking up this dying client
			cm.clients.Remove(clusterKey)

			// Close the physical connection
			if info.aeroClient != nil {
				info.aeroClient.Close()
				slog.Info("Aerospike client closed (idle)",
					slog.Any("hosts", info.aeroClient.Cluster().GetSeeds()),
					slog.Int("len", cm.clients.Size()),
					slog.Any("id", clusterKey),
				)
				info.aeroClient = nil
			}
			info.closeTimer = nil
		}
	})
}
