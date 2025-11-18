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
	// Close ensures that the specified backup client is closed.
	Close(Client)
}

type Cluster interface {
	// Cluster exposes the cluster object to the user
	Cluster() *as.Cluster
}

// ClientManagerImpl implements [ClientManager].
// Is responsible for creating and closing backup clients.
type ClientManagerImpl struct {
	clients       *collections.SafeMap[string, *clientInfo]
	clientFactory ClientFactory
	closeDelay    time.Duration
}

var _ ClientManager = (*ClientManagerImpl)(nil)

type clientInfo struct {
	sync.RWMutex
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

	info, exists := cm.clients.LoadOrStore(clusterKey, newInfo(cluster))
	if exists {
		return cm.checkHealthAndIncrement(ctx, info, label)
	}

	// here, we are the first to create a client for this cluster

	aeroClient, err := cm.clientFactory.NewClientWithPolicyAndHost(cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to aerospike cluster, %w", err)
	}
	info.aeroClient = aeroClient

	return cm.checkHealthAndIncrement(ctx, info, label)
}

func newInfo(cluster *model.AerospikeCluster) *clientInfo {
	value := &clientInfo{}
	if cluster.MaxParallelScans != nil {
		value.scanLimiter = semaphore.NewWeighted(int64(*cluster.MaxParallelScans))
	}
	return value
}

func (cm *ClientManagerImpl) checkHealthAndIncrement(ctx context.Context, info *clientInfo, label string) (Client, error) {
	client, err := cm.createBackupClient(info, label)
	if err != nil {
		return nil, err
	}
	status, err := client.InfoClient().GetStatus(ctx)
	if err != nil {
		return nil, err
	}
	if status != "ok" {
		return nil, errors.New("aerospike cluster connection lost")
	}

	cm.incrementRef(info)
	return client, nil
}

// createClient creates a new backup client given the aerospike cluster configuration.
func (cm *ClientManagerImpl) createBackupClient(info *clientInfo, label string) (Client, error) {
	var options []backup.ClientOpt
	options = append(options, backup.WithScanLimiter(info.scanLimiter))
	if len(label) > 0 {
		options = append(options, backup.WithLogger(slog.With(slog.String("label", label))))
	}

	return cm.clientFactory.NewBackupClient(info.aeroClient, options...)
}

// Close ensures that the specified backup client is closed.
func (cm *ClientManagerImpl) Close(client Client) {
	cm.clients.Iterate(func(id string, info *clientInfo) {
		if info.aeroClient == client.AerospikeClient() {
			cm.decrementRef(info, id)
			return
		}
	})

	// Close client even if it was not found in the cache
	client.AerospikeClient().Close()
	slog.Info("Closed Aerospike client not managed by the cache",
		slog.Any("hosts", client.AerospikeClient().Cluster().GetSeeds()))
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
	info.Lock()
	defer info.Unlock()

	info.count--
	if info.count == 0 {
		info.closeTimer = cm.scheduleClosing(clusterKey)
	}
}

// scheduleClosing schedules client closing after the configured delay.
// Returns a timer that can be used to cancel the scheduled closing if needed.
func (cm *ClientManagerImpl) scheduleClosing(clusterKey string) *time.Timer {
	return time.AfterFunc(cm.closeDelay, func() {
		// Check if the client still exists and count is still 0
		if info, exists := cm.clients.Load(clusterKey); exists && info.count == 0 {
			client := info.aeroClient
			cm.clients.Remove(clusterKey)

			client.Close()
			slog.Info("Aerospike client closed",
				slog.Any("hosts", client.Cluster().GetSeeds()),
				slog.Int("len", cm.clients.Size()),
				slog.Any("id", clusterKey),
			)

			return
		}
	})
}
