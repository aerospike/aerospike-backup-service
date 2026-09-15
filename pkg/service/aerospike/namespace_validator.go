package aerospike

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
)

// NamespaceValidator checks that every configured Aerospike cluster is reachable and that
// routines reference namespaces that exist in their respective source clusters.
// Validation is advisory: unreachable clusters and missing namespaces are reported
// without rejecting configuration.
type NamespaceValidator interface {
	// Validate connects to every cluster in the backup configuration and validates all
	// routines against them.
	Validate(ctx context.Context, backupConfig *model.BackupConfig)
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

func (nv *namespaceValidator) Validate(ctx context.Context, backupConfig *model.BackupConfig) {
	if backupConfig == nil {
		return
	}

	missing := nv.findMissingNamespaces(ctx, backupConfig.AerospikeClusters, backupConfig.BackupRoutines)

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
//
// Clusters are dialed concurrently. They are independent, and one cluster that is down
// would otherwise hold up every cluster behind it for its whole connect timeout.
func (nv *namespaceValidator) fetchNamespacesByCluster(
	ctx context.Context,
	clusters map[string]*model.AerospikeCluster,
) map[*model.AerospikeCluster][]string {
	var (
		mu                  sync.Mutex
		wg                  sync.WaitGroup
		namespacesByCluster = make(map[*model.AerospikeCluster][]string, len(clusters))
	)

	for name, cluster := range clusters {
		wg.Go(func() {
			namespaces, err := nv.fetchClusterNamespaces(ctx, cluster)
			if err != nil {
				// Cancellation means the service is going away, so the failure says
				// nothing about the cluster. A deadline does: the cluster accepted the
				// connection and never answered, which is worth reporting.
				if !errors.Is(ctx.Err(), context.Canceled) {
					slog.Warn("Configured Aerospike cluster is not available",
						slog.String("cluster", name),
						attr.Error(err),
					)
				}

				return
			}

			mu.Lock()
			defer mu.Unlock()
			namespacesByCluster[cluster] = namespaces
		})
	}

	wg.Wait()

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
