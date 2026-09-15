// Package preflight probes the external systems a configuration points at - Aerospike
// clusters, their namespaces, and storage backends - before the service starts scheduling
// work against them.
package preflight

import (
	"context"
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
// It is advisory and never fails: a cluster that is down at startup delays the backups
// that use it, not the service. Checking is therefore an optional step - a caller that
// does not want to pay for the probes simply does not call it.
type Checker interface {
	// Check probes every configured cluster and storage backend, logging one warning
	// per problem found. It returns when every probe has finished.
	Check(ctx context.Context, config *model.Config)

	// CheckChanges probes only what current adds or alters relative to previous.
	// A change that removes something, or that alters only what the service does not
	// have to reach, probes nothing.
	CheckChanges(ctx context.Context, previous, current *model.Config)
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

	backupConfig := config.BackupConfigCopy()
	if len(backupConfig.AerospikeClusters) == 0 && len(backupConfig.Storage) == 0 {
		return // nothing to reach.
	}

	start := time.Now()
	slog.Info("Validating configured clusters and storage")

	var wg sync.WaitGroup

	wg.Go(func() {
		c.clusters.Validate(ctx, config)
	})

	for name, s := range backupConfig.Storage {
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

// CheckChanges probes only what current adds or alters relative to previous: a cluster or
// storage that is new or configured differently, and a routine that now reads a different
// cluster or storage, or names different namespaces.
//
// It replaces the per-handler decision of whether a change is worth validating. A deletion
// adds nothing, so it probes nothing; so does a change that touches only what the service
// never has to reach - a schedule, a policy, a routine being enabled or disabled.
func (c *checker) CheckChanges(ctx context.Context, previous, current *model.Config) {
	c.Check(ctx, changedOnly(previous, current))
}

// changedOnly returns a configuration holding just the entities current adds or alters
// relative to previous.
//
// Comparison is by value, never by pointer: every configuration change round trips through
// the DTO layer, which allocates fresh entities even for the parts nobody touched, so every
// pointer differs on every change. It is also done on the model rather than the DTO, because
// DTO secrets redact themselves - a rotated storage credential would compare equal there,
// which is precisely a change worth probing.
func changedOnly(previous, current *model.Config) *model.Config {
	delta := model.NewConfig()
	if current == nil {
		return delta
	}

	currentBackup := current.BackupConfigCopy()
	previousBackup := model.NewConfig().BackupConfigCopy()
	if previous != nil {
		previousBackup = previous.BackupConfigCopy()
	}

	for name, cluster := range currentBackup.AerospikeClusters {
		if !reflect.DeepEqual(previousBackup.AerospikeClusters[name], cluster) {
			_ = delta.AddCluster(name, cluster)
		}
	}

	for name, entry := range currentBackup.Storage {
		if !reflect.DeepEqual(previousBackup.Storage[name], entry) {
			_ = delta.AddStorage(name, entry)
		}
	}

	// A routine is carried into the delta for the cluster it reads and the storage it
	// writes, separately: repointing a routine at another existing cluster changes nothing
	// about its storage, and editing a cluster changes nothing about the storage of every
	// routine that happens to use it.
	for name, routine := range currentBackup.BackupRoutines {
		previousRoutine := previousBackup.BackupRoutines[name]

		if routineSourceChanged(previousRoutine, routine) {
			_ = delta.AddRoutine(routine)
			// The namespace diff needs the cluster the routine reads, whether or not
			// that cluster changed on its own.
			addNamed(currentBackup.AerospikeClusters, routine.SourceCluster, delta.AddCluster)
		}

		if routineStorageChanged(previousRoutine, routine) {
			addNamed(currentBackup.Storage, routine.Storage, delta.AddStorage)
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

// addNamed copies the entry a routine points at into the delta under the name it carries
// in the full configuration. Adding an entry the delta already holds is a no-op.
func addNamed[T comparable](entries map[string]T, target T, add func(string, T) error) {
	for name, entry := range entries {
		if entry == target {
			_ = add(name, entry)
			return
		}
	}
}
