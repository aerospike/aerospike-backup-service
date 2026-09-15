package aerospike

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
)

// NamespaceValidator checks that every configured Aerospike cluster is reachable and that
// routines reference namespaces that exist in their respective source clusters.
// Validation is advisory: unreachable clusters and missing namespaces are reported
// without rejecting configuration.
type NamespaceValidator interface {
	// Validate connects to every cluster in config and validates all routines against them.
	Validate(ctx context.Context, cfg *model.Config)
}

type namespaceValidator struct {
	clientManager ClientManager
}

var _ NamespaceValidator = (*namespaceValidator)(nil)

func NewNamespaceValidator(cm ClientManager) NamespaceValidator {
	return &namespaceValidator{clientManager: cm}
}

// NamespacesByRoutine stores list of namespaces missing in each routine.
type NamespacesByRoutine map[string][]string

func (nv *namespaceValidator) Validate(ctx context.Context, cfg *model.Config) {
	if cfg == nil {
		return
	}

	missing := nv.findMissingNamespaces(ctx, cfg.BackupConfigCopy().AerospikeClusters, cfg.Routines())

	for routine, namespaces := range missing {
		slog.Warn("Namespaces referenced by routine are missing in the cluster",
			attr.Routine(routine),
			slog.Any("missingNamespaces", namespaces),
		)
	}
}

func (nv *namespaceValidator) findMissingNamespaces(
	ctx context.Context,
	clusters map[string]*model.AerospikeCluster,
	routines map[string]*model.BackupRoutine,
) NamespacesByRoutine {
	namespacesByCluster := nv.fetchNamespacesByCluster(ctx, clusters)
	return nv.diffRoutineNamespaces(routines, namespacesByCluster)
}

// fetchNamespacesByCluster fetches namespaces for each configured cluster. A cluster that
// cannot be reached is reported by name and left out of the result, so that connectivity
// is checked for every cluster in the configuration, not only for the ones a routine uses.
func (nv *namespaceValidator) fetchNamespacesByCluster(
	ctx context.Context,
	clusters map[string]*model.AerospikeCluster,
) map[*model.AerospikeCluster][]string {
	namespacesByCluster := make(map[*model.AerospikeCluster][]string, len(clusters))
	for name, cluster := range clusters {
		namespaces, err := nv.fetchClusterNamespaces(ctx, cluster)
		if err != nil {
			slog.Warn("Configured Aerospike cluster is not available",
				slog.String("cluster", name),
				attr.Error(err),
			)
			continue
		}
		namespacesByCluster[cluster] = namespaces
	}

	return namespacesByCluster
}

// fetchClusterNamespaces gets the namespace list from the given cluster.
func (nv *namespaceValidator) fetchClusterNamespaces(
	ctx context.Context,
	cluster *model.AerospikeCluster,
) ([]string, error) {
	client, err := nv.clientManager.GetClient(ctx, cluster, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to cluster: %w", err)
	}
	defer nv.clientManager.Close(client)

	namespaces, err := client.InfoClient().GetNamespacesList(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve namespaces: %w", err)
	}

	return namespaces, nil
}

// diffRoutineNamespaces returns a map of routines to missing namespaces.
func (nv *namespaceValidator) diffRoutineNamespaces(
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
