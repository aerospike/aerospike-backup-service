package restoreexecutor

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
)

// inRestoreOrder returns the requests of a chain, given oldest first, in the order to restore
// them. A chain is restored oldest first, so that later versions overwrite earlier ones, except
// under the unique (CREATE_ONLY) policy, which keeps the first version written: then newest first,
// so that each record gets its latest version and no generation is bumped for nothing. A chain
// into an empty namespace switches to that policy, since there are no records to keep.
// disable-reordering keeps both the policy and the order. A single backup has no order to change.
func inRestoreOrder(
	ctx context.Context,
	client aerospike.Client,
	logger *slog.Logger,
	requests []*model.RestoreRequest,
) ([]*model.RestoreRequest, error) {
	first := requests[0]
	if len(requests) < 2 || first.DisableReordering {
		return requests, nil
	}

	if first.Policy.IsUniqueOrDefault() {
		return newestFirst(requests), nil
	}

	// The emptiness check decides how existing records are treated, so it must look at the
	// namespace the records are written to, not the one they were backed up from.
	target := first.Policy.Namespace.DestinationOr(first.Namespace)
	count, err := client.InfoClient().GetRecordCount(ctx, target, first.Policy.SetList)
	if err != nil {
		return nil, fmt.Errorf("failed to determine if namespace %s is empty: %w", target, err)
	}

	if count != 0 {
		return requests, nil
	}

	logger.Info("Namespace is empty: switching to the unique policy", slog.String("namespace", target))

	return newestFirst(withUniquePolicy(requests)), nil
}

// newestFirst returns the requests in reverse order.
func newestFirst(requests []*model.RestoreRequest) []*model.RestoreRequest {
	reversed := slices.Clone(requests)
	slices.Reverse(reversed)

	return reversed
}

// withUniquePolicy returns copies of the requests that restore under the unique policy. Replace
// is dropped: it excludes unique, and there are no records to replace.
func withUniquePolicy(requests []*model.RestoreRequest) []*model.RestoreRequest {
	unique := make([]*model.RestoreRequest, 0, len(requests))
	for _, request := range requests {
		request := *request
		request.Policy.Unique = new(true)
		request.Policy.Replace = nil
		unique = append(unique, &request)
	}

	return unique
}
