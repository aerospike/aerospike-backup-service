package service

import (
	"fmt"
	"sync"

	"github.com/aerospike/aerospike-client-go/v7"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup/pkg/model"
	"golang.org/x/sync/semaphore"
)

type BackupClientManager struct {
	clusters map[string]*model.AerospikeCluster
	clients  map[string]*clientInfo
	mu       sync.Mutex
}

type clientInfo struct {
	client *backup.Client
	count  int
}

func NewClientManager(clusters map[string]*model.AerospikeCluster) *BackupClientManager {
	return &BackupClientManager{
		clusters: clusters,
		clients:  make(map[string]*clientInfo),
	}
}

func (cm *BackupClientManager) GetClient(clusterName string) (*backup.Client, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if info, exists := cm.clients[clusterName]; exists {
		info.count++
		return info.client, nil
	}

	cluster, found := cm.clusters[clusterName]
	if !found {
		return nil, fmt.Errorf("cluster %s not found", clusterName)
	}

	client, err := cm.CreateClient(cluster)
	if err != nil {
		return nil, fmt.Errorf("cannot create backup client: %w", err)
	}

	cm.clients[clusterName] = &clientInfo{
		client: client,
		count:  1,
	}

	return client, nil
}

func (cm *BackupClientManager) CreateClient(cluster *model.AerospikeCluster) (*backup.Client, error) {
	aClient, aerr := aerospike.NewClientWithPolicyAndHost(cluster.ASClientPolicy(), cluster.ASClientHosts()...)
	if aerr != nil {
		return nil, fmt.Errorf("failed to connect to aerospike cluster, %w", aerr)
	}

	var options []backup.ClientOpt
	if cluster.MaxParallelScans != nil {
		options = append(options, backup.WithScanLimiter(semaphore.NewWeighted(int64(*cluster.MaxParallelScans))))
	}
	if cluster.ClusterLabel != nil {
		options = append(options, backup.WithID(*cluster.ClusterLabel))
	}

	client, err := backup.NewClient(aClient, options...)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (cm *BackupClientManager) Close(client *backup.Client) {
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

// ClientManager defines the interface for BackupClientManager
type ClientManager interface {
	GetClient(string) (*backup.Client, error)
	CreateClient(*model.AerospikeCluster) (*backup.Client, error)
	Close(*backup.Client)
}
