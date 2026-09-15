// Package preflight probes the external systems a configuration points at - Aerospike
// clusters, their namespaces, and storage backends - before the service starts scheduling
// work against them.
package preflight

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"slices"
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
// It is advisory and never fails: a cluster that is down delays the backups that use it,
// not the service. Nothing reads its result - every finding is a log line - so a caller
// that does not want to wait for the probes runs Check on a goroutine.
type Checker interface {
	// Check probes every cluster and storage backend in backupConfig, logging one
	// warning per problem found. It returns when every probe has finished.
	Check(ctx context.Context, backupConfig *model.BackupConfig)
}

// checkTimeout is a backstop on one whole pass, not a budget for any single probe: every
// probe already carries its own tighter deadline, and they all run concurrently. It exists
// so that a backend which accepts a connection and then never answers cannot pin the
// goroutine - and, on a config change, the context it runs under cannot be canceled at all.
const checkTimeout = 2 * time.Minute

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

// shuttingDown reports whether ctx was canceled rather than merely timed out.
//
// The distinction decides whether a failed probe is worth reporting. Cancellation means
// the service is going away, so the failure says nothing about the backend and a warning
// would be a false alarm. A deadline is the opposite: the backend accepted the connection
// and never answered, which is exactly what the operator needs to be told.
func shuttingDown(ctx context.Context) bool {
	return errors.Is(ctx.Err(), context.Canceled)
}

// nothingToReach reports whether a backup configuration names any external system.
func nothingToReach(backupConfig *model.BackupConfig) bool {
	return backupConfig == nil ||
		(len(backupConfig.AerospikeClusters) == 0 && len(backupConfig.Storage) == 0)
}

// Check probes every cluster and storage backend in backupConfig, logging one warning per
// problem found, and returns when every probe has finished. Probes run concurrently: they
// are independent, and a single unreachable backend would otherwise hold up the whole
// check for its connect timeout.
func (c *checker) Check(ctx context.Context, backupConfig *model.BackupConfig) {
	if nothingToReach(backupConfig) {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	start := time.Now()
	slog.Info("Validating configured clusters and storage")

	var wg sync.WaitGroup

	wg.Go(func() {
		c.clusters.Validate(ctx, backupConfig)
	})

	for name, s := range backupConfig.Storage {
		wg.Go(func() {
			if err := c.storage.Probe(ctx, s); err != nil && !shuttingDown(ctx) {
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

// Changes returns a backup configuration holding just the entities current adds or alters
// relative to previous: a cluster or storage that is new or configured differently, and a
// routine that now reads a different cluster or storage, or names different namespaces.
//
// It replaces the per-handler decision of whether a change is worth validating. A deletion
// adds nothing, so it yields nothing; so does a change that touches only what the service
// never has to reach - a schedule, a policy, a routine being enabled or disabled.
//
// Comparison is by value, never by pointer: every configuration change round trips through
// the DTO layer, which allocates fresh entities even for the parts nobody touched, so every
// pointer differs on every change. It is also done on the model rather than the DTO, because
// DTO secrets redact themselves - a rotated storage credential would compare equal there,
// which is precisely a change worth probing.
func Changes(previous, current *model.BackupConfig) *model.BackupConfig {
	delta := model.NewBackupConfig()
	if current == nil {
		return delta
	}

	if previous == nil {
		previous = model.NewBackupConfig()
	}

	for name, cluster := range current.AerospikeClusters {
		if !reflect.DeepEqual(previous.AerospikeClusters[name], cluster) {
			delta.AerospikeClusters[name] = cluster
		}
	}

	for name, entry := range current.Storage {
		if !reflect.DeepEqual(previous.Storage[name], entry) {
			delta.Storage[name] = entry
		}
	}

	// A routine is carried into the delta for the cluster it reads and the storage it
	// writes, separately: repointing a routine at another existing cluster changes nothing
	// about its storage, and editing a cluster changes nothing about the storage of every
	// routine that happens to use it.
	for name, routine := range current.BackupRoutines {
		previousRoutine := previous.BackupRoutines[name]

		if routineSourceChanged(previousRoutine, routine) {
			delta.BackupRoutines[name] = routine
			// The namespace diff needs the cluster the routine reads, whether or not
			// that cluster changed on its own.
			copyNamed(current.AerospikeClusters, delta.AerospikeClusters, routine.SourceCluster)
		}

		if routineStorageChanged(previousRoutine, routine) {
			copyNamed(current.Storage, delta.Storage, routine.Storage)
		}
	}

	return delta
}

// routineSourceChanged reports whether a routine now reads a different cluster or names
// different namespaces - the two inputs to the namespace diff. A routine's schedule,
// policy, bin list and disabled flag can all change without changing what the check has
// to reach, and the check has nothing to say about those.
func routineSourceChanged(previous, current *model.BackupRoutine) bool {
	if previous == nil {
		return true
	}

	return !slices.Equal(previous.Namespaces, current.Namespaces) ||
		!reflect.DeepEqual(previous.SourceCluster, current.SourceCluster)
}

// routineStorageChanged reports whether a routine now writes somewhere else, which is a
// change worth probing even when that storage is an existing entry nobody edited.
func routineStorageChanged(previous, current *model.BackupRoutine) bool {
	if previous == nil {
		return true
	}

	return !reflect.DeepEqual(previous.Storage, current.Storage)
}

// copyNamed copies the entry a routine points at into the delta under the name it carries
// in the full configuration.
func copyNamed[T comparable](from, to map[string]T, target T) {
	for name, entry := range from {
		if entry == target {
			to[name] = entry
			return
		}
	}
}
