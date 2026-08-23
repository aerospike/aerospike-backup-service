package aerospike

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	_ "github.com/aerospike/aerospike-backup-service/v3/modules/schema" // it's required to load configuration schemas in init method
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/try"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/aerospike/aerospike-management-lib/info"
	"github.com/go-logr/logr"
)

// ClusterConfigSource collects aerospike.conf from live cluster nodes for backup.
type ClusterConfigSource interface {
	// NodeConfigs returns one serialized aerospike.conf per reachable node.
	NodeConfigs(
		ctx context.Context,
		cluster *model.AerospikeCluster,
		logger *slog.Logger,
	) ([]asconfig.DotConf, error)
}

type clusterConfigSource struct {
	clientManager     ClientManager
	readConfiguration func(client Cluster, logger *slog.Logger) []asconfig.DotConf
}

var _ ClusterConfigSource = (*clusterConfigSource)(nil)

// NewClusterConfigSource returns a ClusterConfigSource.
func NewClusterConfigSource(clientManager ClientManager) ClusterConfigSource {
	return &clusterConfigSource{
		clientManager:     clientManager,
		readConfiguration: readConfiguration,
	}
}

func (s *clusterConfigSource) NodeConfigs(
	ctx context.Context,
	clusterCfg *model.AerospikeCluster,
	logger *slog.Logger,
) ([]asconfig.DotConf, error) {
	client, err := s.clientManager.GetClient(ctx, clusterCfg, nil, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to get backup client: %w", err)
	}

	defer s.clientManager.Close(client)

	infos := s.readConfiguration(client.AerospikeClient(), logger)
	if len(infos) == 0 {
		return nil, errors.New("failed to read Aerospike configuration")
	}

	return infos, nil
}

func readConfiguration(client Cluster, logger *slog.Logger) []asconfig.DotConf {
	activeHosts := getActiveHosts(clusterNodesOf(client))
	if len(activeHosts) == 0 {
		return nil
	}

	var outputs = make([]asconfig.DotConf, 0, len(activeHosts))

	policy := client.Cluster().ClientPolicy()
	for _, host := range activeHosts {
		asInfo := info.NewAsInfo(logr.Logger{}, host, &policy)

		conf, err := try.RecoverError(func() (*asconfig.GenConf, error) {
			return asconfig.GenerateConf(logr.Discard(), asInfo, true)
		})
		if err != nil {
			logger.Error("Error reading configuration",
				slog.Any("host", host), attr.Error(err))
			continue
		}

		asconf, err := try.RecoverError(func() (*asconfig.AsConfig, error) {
			return asconfig.NewMapAsConfig(logr.Discard(), conf.Conf)
		})
		if err != nil {
			logger.Error("Error parsing configuration",
				slog.Any("host", host), attr.Error(err))
			continue
		}

		configAsString, err := try.Recover(asconf.ToConfFile)
		if err != nil {
			logger.Error("Error serializing configuration",
				slog.Any("host", host), attr.Error(err))
			continue
		}

		outputs = append(outputs, configAsString)
	}

	return outputs
}

// clusterNode is the subset of as.Node used when collecting aerospike.conf.
type clusterNode interface {
	IsActive() bool
	GetHost() *as.Host
}

func clusterNodesOf(client Cluster) []clusterNode {
	raw := client.Cluster().GetNodes()
	nodes := make([]clusterNode, len(raw))
	for i, node := range raw {
		nodes[i] = node
	}

	return nodes
}

func getActiveHosts(nodes []clusterNode) []*as.Host {
	var activeHosts []*as.Host
	for _, node := range nodes {
		if node.IsActive() {
			activeHosts = append(activeHosts, node.GetHost())
		}
	}

	return activeHosts
}
