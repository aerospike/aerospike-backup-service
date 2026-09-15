package preflight

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestChecker_AllReachable_NoWarnings(t *testing.T) {
	ctrl := gomock.NewController(t)
	warnings := captureWarnings(t)

	config := configWithStorage(t, "s1", "s2")

	clusters := aerospike.NewMockNamespaceValidator(ctrl)
	clusters.EXPECT().Validate(gomock.Any(), config)

	ops := storage.NewMockOperations(ctrl)
	ops.EXPECT().Probe(gomock.Any(), gomock.Any()).Return(nil).Times(2)

	NewChecker(clusters, ops).Check(t.Context(), config)

	assert.Empty(t, warnings.messages())
}

// An unreachable storage is reported by name and does not stop the check: the point of
// the step is that every problem in the configuration is on screen before the service
// starts, not that the first one aborts it.
func TestChecker_UnreachableStorage_WarnsPerStorage(t *testing.T) {
	ctrl := gomock.NewController(t)
	warnings := captureWarnings(t)

	config := configWithStorage(t, "broken-1", "broken-2")

	clusters := aerospike.NewMockNamespaceValidator(ctrl)
	clusters.EXPECT().Validate(gomock.Any(), config)

	ops := storage.NewMockOperations(ctrl)
	ops.EXPECT().
		Probe(gomock.Any(), gomock.Any()).
		Return(errors.New("bucket does not exist")).
		Times(2)

	NewChecker(clusters, ops).Check(t.Context(), config)

	names := warnings.attrValues("storage")
	assert.ElementsMatch(t, []string{"broken-1", "broken-2"}, names)
	for _, msg := range warnings.messages() {
		assert.Equal(t, "Configured storage is not available", msg)
	}
}

// Clusters are checked even when no storage is configured: neither half of the check is
// conditional on the other.
func TestChecker_ChecksClustersWithoutStorage(t *testing.T) {
	ctrl := gomock.NewController(t)

	config := model.NewConfig()
	require.NoError(t, config.AddCluster("cluster1", &model.AerospikeCluster{}))
	backupConfig := config.BackupConfigCopy()

	clusters := aerospike.NewMockNamespaceValidator(ctrl)
	clusters.EXPECT().Validate(gomock.Any(), backupConfig)

	NewChecker(clusters, storage.NewMockOperations(ctrl)).Check(t.Context(), backupConfig)
}

// There is nothing to say about a configuration that points at nothing, and a delta is
// empty far more often than not - every deletion and every schedule edit produces one.
func TestChecker_NothingToReach_DoesNothing(t *testing.T) {
	ctrl := gomock.NewController(t)

	// No EXPECT calls: neither collaborator may be touched.
	NewChecker(aerospike.NewMockNamespaceValidator(ctrl), storage.NewMockOperations(ctrl)).
		Check(t.Context(), model.NewBackupConfig())
}

func TestChecker_NilConfig_DoesNothing(t *testing.T) {
	ctrl := gomock.NewController(t)

	// No EXPECT calls: a nil config must not reach either collaborator.
	NewChecker(aerospike.NewMockNamespaceValidator(ctrl), storage.NewMockOperations(ctrl)).
		Check(t.Context(), nil)
}

// Storage probes have to overlap: one unreachable backend blocks for its whole connect
// timeout, and a sequential check would pay that timeout once per storage.
func TestChecker_ProbesRunConcurrently(t *testing.T) {
	ctrl := gomock.NewController(t)

	config := configWithStorage(t, "s1", "s2", "s3")

	clusters := aerospike.NewMockNamespaceValidator(ctrl)
	clusters.EXPECT().Validate(gomock.Any(), gomock.Any())

	// Every probe waits for all the others, so the check only completes if they overlap.
	var arrived sync.WaitGroup
	arrived.Add(3)

	ops := storage.NewMockOperations(ctrl)
	ops.EXPECT().
		Probe(gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, model.Storage) error {
			arrived.Done()
			arrived.Wait()

			return nil
		}).
		Times(3)

	NewChecker(clusters, ops).Check(t.Context(), config)
}

func configWithStorage(t *testing.T, names ...string) *model.BackupConfig {
	t.Helper()

	config := model.NewConfig()
	for _, name := range names {
		require.NoError(t, config.AddStorage(name, &model.LocalStorage{Path: t.TempDir()}))
	}

	return config.BackupConfigCopy()
}

// recorder collects the warnings the checker writes to the default logger.
type recorder struct {
	slog.Handler

	mu      sync.Mutex
	records []slog.Record
}

func captureWarnings(t *testing.T) *recorder {
	t.Helper()

	r := &recorder{Handler: slog.DiscardHandler}
	previous := slog.Default()
	slog.SetDefault(slog.New(r))
	t.Cleanup(func() { slog.SetDefault(previous) })

	return r
}

func (r *recorder) Enabled(context.Context, slog.Level) bool { return true }

func (r *recorder) Handle(_ context.Context, record slog.Record) error {
	if record.Level < slog.LevelWarn {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, record)

	return nil
}

func (r *recorder) messages() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	messages := make([]string, 0, len(r.records))
	for _, record := range r.records {
		messages = append(messages, record.Message)
	}

	return messages
}

// attrValues returns the value of the named attribute of every recorded warning.
func (r *recorder) attrValues(key string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	values := make([]string, 0, len(r.records))
	for _, record := range r.records {
		record.Attrs(func(a slog.Attr) bool {
			if a.Key == key {
				values = append(values, a.Value.String())
			}

			return true
		})
	}

	return values
}

// Probes must run under a deadline of the checker's own. A config change hands Check a
// context whose cancellation has been stripped, so without this a backend that accepts a
// connection and never answers would pin the goroutine for the life of the process.
func TestChecker_BoundsProbesWithADeadline(t *testing.T) {
	ctrl := gomock.NewController(t)

	clusters := aerospike.NewMockNamespaceValidator(ctrl)
	var clusterDeadline bool
	clusters.EXPECT().
		Validate(gomock.Any(), gomock.Any()).
		Do(func(ctx context.Context, _ *model.BackupConfig) {
			_, clusterDeadline = ctx.Deadline()
		})

	ops := storage.NewMockOperations(ctrl)
	var storageDeadline bool
	ops.EXPECT().
		Probe(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, _ model.Storage) error {
			_, storageDeadline = ctx.Deadline()

			return nil
		})

	// An uncancellable context, as a config change supplies.
	newTestChecker(clusters, ops).Check(context.WithoutCancel(t.Context()), configWithStorage(t, "s1"))

	assert.True(t, storageDeadline, "the storage probe ran without a deadline")
	assert.True(t, clusterDeadline, "the cluster check ran without a deadline")
}

// newTestChecker returns the concrete checker so a test can drive one pass directly.
func newTestChecker(clusters aerospike.NamespaceValidator, operations storage.Operations) *checker {
	return NewChecker(clusters, operations).(*checker)
}
