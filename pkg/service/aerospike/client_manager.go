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
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/syncutil"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/aerospike/backup-go"
	"golang.org/x/sync/semaphore"
)

// ClientManager hands out backup clients for an Aerospike cluster. The underlying connection is
// shared per cluster and reference counted: it is opened on first use and closed shortly after
// the last Close. Connections and client wrappers are created by [ClientFactory].
type ClientManager interface {
	// GetClient returns a backup client by aerospike cluster name (new or cached).
	// localLimiter is an optional per-routine scan limiter. When provided, it is combined
	// with the global cluster limiter using a DualLimiter to enforce both limits.
	GetClient(
		ctx context.Context,
		cluster *model.AerospikeCluster,
		localLimiter syncutil.Limiter,
		logger *slog.Logger,
	) (Client, error)
	// Close ensures that the specified backup client is released (ref count decremented).
	Close(Client)
}

// Cluster exposes the Aerospike cluster object behind a live client.
type Cluster interface {
	// Cluster returns the cluster object of the live connection.
	Cluster() *as.Cluster
}

// clientManager shares one connection per cluster: the first GetClient opens it, later calls
// reuse it, and Close decrements a reference counter. The connection is closed once the counter
// reaches zero and closeDelay has passed.
type clientManager struct {
	// clients holds the state for each cluster.
	clients       *collections.SafeMap[uint64, *clientInfo]
	clientFactory ClientFactory
	closeDelay    time.Duration
}

var _ ClientManager = (*clientManager)(nil)

const DefaultCloseDelay = 10 * time.Second

type clientInfo struct {
	// mu protects the fields below (count, closeTimer, aeroClient)
	mu          sync.RWMutex
	aeroClient  backup.AerospikeClient
	scanLimiter syncutil.Limiter

	count      int
	closeTimer *time.Timer
}

// NewClientManager creates a ClientManager.
// closeDelay specifies how long to wait before actually closing the client after the last user releases it.
func NewClientManager(aerospikeClientFactory ClientFactory, closeDelay time.Duration) ClientManager {
	return &clientManager{
		clients:       collections.NewSafeMap[uint64, *clientInfo](),
		clientFactory: aerospikeClientFactory,
		closeDelay:    closeDelay,
	}
}

// GetClient returns a backup client by aerospike cluster name (new or cached).
// The returned client must be closed by calling Close().
// localLimiter is an optional per-routine scan limiter. When provided, it is combined
// with the global cluster limiter using a DualLimiter to enforce both limits.
// logger will be passed to the backup client. If not set, a default logger will be used.
func (cm *clientManager) GetClient(
	ctx context.Context,
	cluster *model.AerospikeCluster,
	localLimiter syncutil.Limiter,
	logger *slog.Logger,
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
		aeroClient, err := cm.clientFactory.NewClientWithPolicyAndHost(ctx, cluster)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to aerospike cluster: %w", err)
		}
		info.aeroClient = aeroClient
	}

	client, err := cm.createBackupClient(info, localLimiter, logger)
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
		return nil, fmt.Errorf("aerospike cluster connection lost: %s", status)
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

func (cm *clientManager) createBackupClient(
	info *clientInfo,
	localLimiter syncutil.Limiter,
	logger *slog.Logger,
) (Client, error) {
	if logger == nil {
		logger = slog.Default()
	}
	options := []backup.ClientOpt{
		backup.WithInfoPolicies(as.NewInfoPolicy(), model.InfoRetryPolicy),
		backup.WithLogger(logger),
		backup.WithScanLimiter(syncutil.NewDualLimiter(info.scanLimiter, localLimiter)),
	}

	return cm.clientFactory.NewBackupClient(info.aeroClient, options...)
}

// Close ensures that the specified backup client is released.
func (cm *clientManager) Close(client Client) {
	var (
		targetInfo *clientInfo
		targetKey  uint64
	)

	// We need to find which info struct owns this client.
	// Since Client interface wraps the underlying AerospikeClient, we compare pointers.
	found := false
	cm.clients.Iterate(func(key uint64, info *clientInfo) {
		if found {
			return // Optimization: stop if already found
		}

		// We must lock to read info.aeroClient safely,
		// just in case it's being modified (though unlikely after init).
		info.mu.RLock()
		if info.aeroClient == client.AerospikeClient() {
			targetInfo = info
			targetKey = key
			found = true
		}
		info.mu.RUnlock()
	})

	if found {
		cm.decrementRef(targetInfo, targetKey)
	} else {
		// If it's not in our cache, we must close it immediately because we aren't managing its lifecycle.
		client.AerospikeClient().Close()
		slog.Info("Closed Aerospike client not managed by the cache",
			slog.Any("hosts", client.AerospikeClient().Cluster().GetSeeds()))
	}
}

// decrementRef decreases the reference count and schedules closing if count reaches zero.
func (cm *clientManager) decrementRef(info *clientInfo, clusterKey uint64) {
	info.mu.Lock()
	defer info.mu.Unlock()

	info.count--
	if info.count == 0 {
		// If a timer is already running (edge case), stop it first
		if info.closeTimer != nil {
			info.closeTimer.Stop()
		}
		info.closeTimer = cm.scheduleClosing(clusterKey)
	}
}

// scheduleClosing schedules client closing after the configured delay.
func (cm *clientManager) scheduleClosing(clusterKey uint64) *time.Timer {
	return time.AfterFunc(cm.closeDelay, func() {
		// 1. Retrieve the info struct
		info, exists := cm.clients.Load(clusterKey)
		if !exists {
			return
		}

		// 2. Lock to check state safely
		info.mu.Lock()
		defer info.mu.Unlock()

		// 3. Verify condition: Is count still 0?
		// Someone might have called GetClient() before this lock was acquired.
		if info.count == 0 {
			// Remove from map to prevent new users from picking up this dying client
			cm.clients.Remove(clusterKey)

			// Close the physical connection
			if info.aeroClient != nil {
				info.aeroClient.Close()
				slog.Info("Aerospike client closed (idle)",
					slog.Int("len", cm.clients.Size()),
					slog.Any("id", clusterKey),
				)
				info.aeroClient = nil
			}
			info.closeTimer = nil
		}
	})
}
