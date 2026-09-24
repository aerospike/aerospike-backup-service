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
	// clients holds the live entry of each cluster, keyed by the cluster's hash. The first
	// GetClient for a cluster stores its entry; whichever call closes the entry removes it.
	clients       *collections.SafeMap[uint64, *clientInfo]
	clientFactory ClientFactory
	closeDelay    time.Duration
}

var _ ClientManager = (*clientManager)(nil)

const DefaultCloseDelay = 10 * time.Second

// errClientInfoClosed reports that an entry was closed before the caller got to it. The caller
// fetches a fresh entry from the map and tries again.
var errClientInfoClosed = errors.New("client entry is closed")

// managedClient is what GetClient hands out: the backup client together with the entry it was
// taken from, so Close releases exactly that entry without searching the map.
type managedClient struct {
	Client
	info *clientInfo
}

// clientInfo is the shared, reference-counted connection of one cluster. Its methods are the
// only place mu is taken: the manager stores entries in the map and removes the ones that
// report they closed.
type clientInfo struct {
	key         uint64
	cluster     *model.AerospikeCluster
	factory     ClientFactory
	scanLimiter syncutil.Limiter

	// mu protects the fields below.
	mu         sync.Mutex
	aeroClient backup.AerospikeClient
	count      int
	closeTimer *time.Timer
	// closed is set when the connection is gone for good. A closed entry is never reused: a
	// caller that still holds it gets errClientInfoClosed and fetches a fresh one.
	closed bool
}

func newClientInfo(key uint64, cluster *model.AerospikeCluster, factory ClientFactory) *clientInfo {
	info := &clientInfo{key: key, cluster: cluster, factory: factory}
	if cluster.MaxParallelScans != nil {
		info.scanLimiter = semaphore.NewWeighted(int64(*cluster.MaxParallelScans))
	}

	return info
}

// acquire connects on first use, checks that the connection is healthy and takes a reference.
// A failed check leaves the connection in place for the callers that already hold it; the
// caller decides whether to drop it with closeIfUnused.
func (info *clientInfo) acquire(
	ctx context.Context,
	localLimiter syncutil.Limiter,
	logger *slog.Logger,
) (Client, error) {
	info.mu.Lock()
	defer info.mu.Unlock()

	if info.closed {
		return nil, errClientInfoClosed
	}

	if info.aeroClient == nil {
		aeroClient, err := info.factory.NewClientWithPolicyAndHost(ctx, info.cluster)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to aerospike cluster: %w", err)
		}
		info.aeroClient = aeroClient
	}

	client, err := info.newBackupClient(localLimiter, logger)
	if err != nil {
		return nil, err
	}

	status, err := client.InfoClient().GetStatus(ctx)
	if err != nil {
		return nil, err
	}
	if status != "ok" {
		return nil, fmt.Errorf("aerospike cluster connection lost: %s", status)
	}

	if info.closeTimer != nil {
		info.closeTimer.Stop()
		info.closeTimer = nil
	}
	info.count++

	return client, nil
}

// newBackupClient wraps the connection for one caller. The caller must hold mu.
func (info *clientInfo) newBackupClient(localLimiter syncutil.Limiter, logger *slog.Logger) (Client, error) {
	if logger == nil {
		logger = slog.Default()
	}
	options := []backup.ClientOpt{
		backup.WithInfoPolicies(as.NewInfoPolicy(), model.InfoRetryPolicy),
		backup.WithLogger(logger),
		backup.WithScanLimiter(syncutil.NewDualLimiter(info.scanLimiter, localLimiter)),
	}

	return info.factory.NewBackupClient(info.aeroClient, options...)
}

// release drops one reference. When the last one goes, onIdle runs after closeDelay unless a
// new reference is taken first.
func (info *clientInfo) release(closeDelay time.Duration, onIdle func()) {
	info.mu.Lock()
	defer info.mu.Unlock()

	info.count--
	if info.count > 0 {
		return
	}

	if info.closeTimer != nil {
		info.closeTimer.Stop()
	}
	info.closeTimer = time.AfterFunc(closeDelay, onIdle)
}

// closeIfUnused closes the connection unless a caller still holds it, and reports whether this
// call closed it. An entry closes at most once, so exactly one caller ever sees true.
func (info *clientInfo) closeIfUnused() bool {
	info.mu.Lock()
	defer info.mu.Unlock()

	if info.closed || info.count > 0 {
		return false
	}

	if info.closeTimer != nil {
		info.closeTimer.Stop()
		info.closeTimer = nil
	}
	if info.aeroClient != nil {
		info.aeroClient.Close()
		info.aeroClient = nil
	}
	info.closed = true

	return true
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

	// An entry closed between LoadOrStore and acquire is on its way out of the map: its closer
	// removes it right after closing it, so the next round gets a fresh one.
	for {
		info := cm.clients.LoadOrStore(clusterKey, newClientInfo(clusterKey, cluster, cm.clientFactory))

		client, err := info.acquire(ctx, localLimiter, logger)
		if errors.Is(err, errClientInfoClosed) {
			continue
		}
		if err != nil {
			if cm.forget(info) {
				slog.Warn("Aerospike client dropped", slog.Any("id", clusterKey), slog.Any("error", err))
			}

			return nil, err
		}

		return managedClient{Client: client, info: info}, nil
	}
}

// forget removes info from the map once nobody holds it, and reports whether it did. Only the
// call that closes an entry removes it, and a key is taken only while it is free, so the entry
// under info.key at that moment is always info itself and never a newer replacement.
func (cm *clientManager) forget(info *clientInfo) bool {
	if !info.closeIfUnused() {
		return false
	}

	cm.clients.Remove(info.key)

	return true
}

// Close ensures that the specified backup client is released.
func (cm *clientManager) Close(client Client) {
	managed, ok := client.(managedClient)
	if !ok {
		// The manager did not hand this client out, so nothing counts its references: close it now.
		aeroClient := client.AerospikeClient()
		aeroClient.Close()
		slog.Info("Closed Aerospike client not managed by the cache",
			slog.Any("hosts", aeroClient.Cluster().GetSeeds()))

		return
	}

	managed.info.release(cm.closeDelay, func() {
		if cm.forget(managed.info) {
			slog.Info("Aerospike client closed (idle)",
				slog.Int("len", cm.clients.Size()),
				slog.Any("id", managed.info.key),
			)
		}
	})
}
