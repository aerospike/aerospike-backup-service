// Package preflight probes the external systems a configuration points at - Aerospike
// clusters, their namespaces, and storage backends - before the service starts scheduling
// work against them.
package preflight

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
)

// Checker reports which of the configured clusters, namespaces and storage backends
// cannot be reached.
//
// It is advisory and never fails: a cluster that is down at startup delays the backups
// that use it, not the service. Checking is therefore an optional step - a caller that
// does not want to pay for the probes simply does not call it.
type Checker interface {
	// Check probes every configured cluster and storage backend, logging one warning
	// per problem found. It returns when every probe has finished.
	Check(ctx context.Context, config *model.Config)
}

type checker struct {
	clusters aerospike.NamespaceValidator
	storage  storage.Operations
}

var _ Checker = (*checker)(nil)

// NewChecker returns a Checker that validates clusters through the given namespace
// validator and storage through the given storage operations.
func NewChecker(clusters aerospike.NamespaceValidator, operations storage.Operations) Checker {
	return &checker{
		clusters: clusters,
		storage:  operations,
	}
}

// Check probes every configured cluster and storage backend, logging one warning per
// problem found. Probes run concurrently: they are independent, and a single unreachable
// backend would otherwise hold up the whole check for its connect timeout.
func (c *checker) Check(ctx context.Context, config *model.Config) {
	if config == nil {
		return
	}

	start := time.Now()
	slog.Info("Validating configured clusters and storage")

	var wg sync.WaitGroup

	wg.Go(func() {
		c.clusters.Validate(ctx, config)
	})

	for name, s := range config.BackupConfigCopy().Storage {
		wg.Go(func() {
			if err := c.storage.Probe(ctx, s); err != nil {
				slog.Warn("Configured storage is not available",
					slog.String("storage", name),
					attr.Error(err),
				)
			}
		})
	}

	wg.Wait()

	slog.Info("Finished validating configured clusters and storage",
		slog.Duration("duration", time.Since(start)))
}
