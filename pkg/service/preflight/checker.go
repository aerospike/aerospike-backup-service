// Package preflight probes the external systems a configuration points at - Aerospike
// clusters, their namespaces, and storage backends - before the service starts scheduling
// work against them.
package preflight

import (
	"context"
	"log/slog"
	"maps"
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
// not the service. Nothing reads its result - every finding is a log line - so probing is
// never done on a caller's goroutine. Requests are queued and served by Start.
type Checker interface {
	// RequestCheck queues a check of whatever current adds or alters relative to
	// previous, and returns at once. A change that removes something, or that alters
	// only what the service does not have to reach, queues nothing.
	//
	// The delta is computed here, in the caller's goroutine, so that a caller holding
	// the configuration lock hands over a snapshot nothing can still be changing.
	// Requests made before Start accumulate, so nothing is probed until the service runs.
	RequestCheck(previous, current *model.BackupConfig)

	// Start serves queued checks until ctx is canceled.
	Start(ctx context.Context)
}

type checker struct {
	clusters aerospike.NamespaceValidator
	storage  storage.Operations

	// pending holds the entities queued checks still have to reach. Deltas merge by
	// name, so back-to-back changes to one storage probe it once, with the newest
	// definition. signal has a buffer of one, so a request never blocks and repeated
	// requests collapse into a single pass.
	pendingMu sync.Mutex
	pending   *model.BackupConfig
	signal    chan struct{}
}

var _ Checker = (*checker)(nil)

// NewChecker returns a Checker that validates clusters through the given namespace
// validator and storage through the given storage operations.
func NewChecker(clusters aerospike.NamespaceValidator, operations storage.Operations) Checker {
	return &checker{
		clusters: clusters,
		storage:  operations,
		pending:  model.NewBackupConfig(),
		signal:   make(chan struct{}, 1),
	}
}

// RequestCheck queues a check of what current adds or alters relative to previous.
func (c *checker) RequestCheck(previous, current *model.BackupConfig) {
	delta := Changes(previous, current)
	if nothingToReach(delta) {
		return
	}

	c.pendingMu.Lock()
	maps.Copy(c.pending.AerospikeClusters, delta.AerospikeClusters)
	maps.Copy(c.pending.Storage, delta.Storage)
	maps.Copy(c.pending.BackupRoutines, delta.BackupRoutines)
	c.pendingMu.Unlock()

	select {
	case c.signal <- struct{}{}:
	default: // a check is already due; it will pick this up.
	}
}

// Start serves queued checks until ctx is canceled.
func (c *checker) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.signal:
				c.check(ctx, c.takePending())
			}
		}
	}()
}

// takePending drains the queue, leaving an empty one behind.
func (c *checker) takePending() *model.BackupConfig {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()

	pending := c.pending
	c.pending = model.NewBackupConfig()

	return pending
}

// nothingToReach reports whether a backup configuration names any external system.
func nothingToReach(backupConfig *model.BackupConfig) bool {
	return backupConfig == nil ||
		(len(backupConfig.AerospikeClusters) == 0 && len(backupConfig.Storage) == 0)
}

// check probes every cluster and storage backend in backupConfig, logging one warning per
// problem found, and returns when every probe has finished. Probes run concurrently: they
// are independent, and a single unreachable backend would otherwise hold up the whole
// check for its connect timeout.
func (c *checker) check(ctx context.Context, backupConfig *model.BackupConfig) {
	if nothingToReach(backupConfig) {
		return
	}

	start := time.Now()
	slog.Info("Validating configured clusters and storage")

	var wg sync.WaitGroup

	wg.Go(func() {
		c.clusters.Validate(ctx, backupConfig)
	})

	for name, s := range backupConfig.Storage {
		wg.Go(func() {
			if err := c.storage.Probe(ctx, s); err != nil && ctx.Err() == nil {
				// A probe that failed because ctx was canceled says nothing about the
				// storage: the service is shutting down, and a warning here would be a
				// false alarm the operator cannot act on.
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
