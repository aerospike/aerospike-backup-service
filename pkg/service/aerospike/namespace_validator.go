package aerospike

import (
	"fmt"
	"log/slog"

	"github.com/aerospike/aerospike-backup-service/v3/internal/util/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
)

// NamespaceValidator provides methods for checking namespace existence in Aerospike clusters.
// NOTE: This implementation is non-blocking: it logs warnings but does not return errors.
type NamespaceValidator interface {
	// ValidateConfig scans all backup routines and logs warnings for missing namespaces in corresponding clusters.
	ValidateConfig(cfg *model.Config)
}

// NamespaceValidatorImpl implements NamespaceValidator.
type NamespaceValidatorImpl struct {
	clientManager ClientManager
	infoRequest   InfoRequest
}

func NewNamespaceValidator(clientManager ClientManager, infoRequest InfoRequest) NamespaceValidator {
	return &NamespaceValidatorImpl{
		clientManager: clientManager,
		infoRequest:   infoRequest,
	}
}

func (nv *NamespaceValidatorImpl) ValidateConfig(cfg *model.Config) {
	if cfg == nil {
		return
	}

	clusters := nv.collectClusters(cfg)
	namespacesByCluster := nv.buildClusterNamespaceCache(clusters)
	nv.validateRoutines(cfg, namespacesByCluster)
}

// collectClusters gathers unique clusters referenced by routines that actually need validation.
func (nv *NamespaceValidatorImpl) collectClusters(cfg *model.Config) map[*model.AerospikeCluster]struct{} {
	clusters := make(map[*model.AerospikeCluster]struct{})
	for _, r := range cfg.Routines() {
		if len(r.Namespaces) == 0 {
			continue
		}

		clusters[r.SourceCluster] = struct{}{}
	}

	return clusters
}

// buildClusterNamespaceCache fetches namespaces once per cluster, logging (warn-only) on failures.
func (nv *NamespaceValidatorImpl) buildClusterNamespaceCache(
	clusters map[*model.AerospikeCluster]struct{},
) map[*model.AerospikeCluster][]string {
	cache := make(map[*model.AerospikeCluster][]string, len(clusters))
	for c := range clusters {
		ns, err := nv.fetchNamespacesForCluster(c)
		if err != nil {
			slog.Error("failed to fetch namespaces for cluster", attr.Error(err))
			continue
		}
		cache[c] = ns
	}

	return cache
}

// fetchNamespacesForCluster obtains the namespace list for a single cluster.
func (nv *NamespaceValidatorImpl) fetchNamespacesForCluster(cluster *model.AerospikeCluster) ([]string, error) {
	client, err := nv.clientManager.GetClient(cluster)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to cluster: %w", err)
	}
	defer nv.clientManager.Close(client)

	ns, err := nv.infoRequest.Namespaces(client.AerospikeClient().Cluster())
	if err != nil {
		return nil, fmt.Errorf("cannot retrieve namespaces: %w", err)
	}

	return ns, nil
}

// validateRoutines checks each routine against the cached namespace list (if present).
func (nv *NamespaceValidatorImpl) validateRoutines(
	cfg *model.Config,
	cache map[*model.AerospikeCluster][]string,
) {
	for name, r := range cfg.Routines() {
		if len(r.Namespaces) == 0 {
			continue
		}

		nsList, ok := cache[r.SourceCluster]
		if !ok {
			// No cache entry -> cluster unreachable / fetch failed; earlier step already warned.
			continue
		}

		missing := util.MissingElements(nsList, r.Namespaces)
		if len(missing) > 0 {
			slog.Warn("namespaces referenced by routine are missing in the cluster",
				attr.Routine(name),
				slog.Any("missing", missing),
			)
		}
	}
}
