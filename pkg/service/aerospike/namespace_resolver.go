package aerospike

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

type NamespaceResolver interface {
	// ResolveNamespaces returns the list of namespaces to back up for the routine.
	ResolveNamespaces(
		ctx context.Context,
		backupRoutine *model.BackupRoutine,
		logger *slog.Logger,
	) ([]string, error)
}

type clusterNamespaceResolver struct {
	clientManager ClientManager
}

func NewNamespaceResolver(clientManager ClientManager) NamespaceResolver {
	return &clusterNamespaceResolver{
		clientManager: clientManager,
	}
}

func (r *clusterNamespaceResolver) ResolveNamespaces(
	ctx context.Context,
	backupRoutine *model.BackupRoutine,
	logger *slog.Logger,
) ([]string, error) {
	if len(backupRoutine.Namespaces) > 0 {
		return backupRoutine.Namespaces, nil
	}

	client, err := r.clientManager.GetClient(ctx, backupRoutine.SourceCluster, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to get backup client: %w", err)
	}
	defer r.clientManager.Close(client)

	namespaces, err := client.InfoClient().GetNamespacesList(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve namespaces from source cluster: %w", err)
	}

	return namespaces, nil
}
