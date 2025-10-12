package aerospike

import (
	"fmt"
	"log/slog"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
)

// NamespaceValidator checks whether routines reference namespaces
// that exist in their respective Aerospike source clusters.
// This implementation logs warnings but never returns errors.
type NamespaceValidator interface {
	Validate(cfg *model.Config)
}

// NamespaceValidatorImpl implements NamespaceValidator.
type NamespaceValidatorImpl struct {
	clientManager ClientManager
}

func NewNamespaceValidator(cm ClientManager) NamespaceValidator {
	return &NamespaceValidatorImpl{clientManager: cm}
}

// NamespacesByRoutine stores list of namespaces missing in each routine.
type NamespacesByRoutine map[string][]string

func (nv *NamespaceValidatorImpl) Validate(cfg *model.Config) {
	if cfg == nil {
		return
	}

	missing := nv.findMissingNamespaces(cfg.Routines())

	for routine, namespaces := range missing {
		slog.Warn("namespaces referenced by routine are missing in the cluster",
			attr.Routine(routine),
			slog.Any("missingNamespaces", namespaces),
		)
	}
}

func (nv *NamespaceValidatorImpl) findMissingNamespaces(routines map[string]*model.BackupRoutine) NamespacesByRoutine {
	clusters := nv.collectClusters(routines)
	namespacesByCluster := nv.fetchNamespacesByCluster(clusters)
	return nv.diffRoutineNamespaces(routines, namespacesByCluster)
}

// collectClusters gathers unique clusters referenced by routines that actually need validation.
func (nv *NamespaceValidatorImpl) collectClusters(
	routines map[string]*model.BackupRoutine,
) map[*model.AerospikeCluster]struct{} {
	clusters := make(map[*model.AerospikeCluster]struct{})
	for _, r := range routines {
		if len(r.Namespaces) > 0 {
			clusters[r.SourceCluster] = struct{}{}
		}
	}

	return clusters
}

// fetchNamespacesByCluster fetches namespaces for each cluster.
func (nv *NamespaceValidatorImpl) fetchNamespacesByCluster(
	clusters map[*model.AerospikeCluster]struct{},
) map[*model.AerospikeCluster][]string {
	namespacesByCluster := make(map[*model.AerospikeCluster][]string, len(clusters))
	for cluster := range clusters {
		namespaces, err := nv.fetchClusterNamespaces(cluster)
		if err != nil {
			slog.Error("failed to fetch namespaces for cluster", attr.Error(err))
			continue
		}
		namespacesByCluster[cluster] = namespaces
	}

	return namespacesByCluster
}

// fetchClusterNamespaces gets the namespace list from the given cluster.
func (nv *NamespaceValidatorImpl) fetchClusterNamespaces(cluster *model.AerospikeCluster) ([]string, error) {
	client, err := nv.clientManager.GetClient(cluster)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to cluster: %w", err)
	}
	defer nv.clientManager.Close(client)

	namespaces, err := client.InfoClient().GetNamespacesList()
	if err != nil {
		return nil, fmt.Errorf("cannot retrieve namespaces: %w", err)
	}

	return namespaces, nil
}

// diffRoutineNamespaces returns a map of routines to missing namespaces.
func (nv *NamespaceValidatorImpl) diffRoutineNamespaces(
	routines map[string]*model.BackupRoutine,
	namespacesByCluster map[*model.AerospikeCluster][]string,
) NamespacesByRoutine {
	result := make(NamespacesByRoutine)
	for name, r := range routines {
		if len(r.Namespaces) == 0 {
			continue
		}
		clusterNamespaces, ok := namespacesByCluster[r.SourceCluster]
		if !ok {
			continue // no data for this cluster; warning already logged
		}

		missing := collections.MissingElements(r.Namespaces, clusterNamespaces)
		if len(missing) > 0 {
			result[name] = missing
		}
	}

	return result
}
