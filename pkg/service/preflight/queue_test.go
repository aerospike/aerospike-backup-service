package preflight

import (
	"context"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// A request made before Start must not reach out to anything: the startup check is queued
// while the object graph is still being built, and nothing may probe until the service runs.
func TestRequestCheck_BeforeStart_ProbesNothing(t *testing.T) {
	ctrl := gomock.NewController(t)

	// No EXPECT calls: touching either collaborator fails the test.
	c := newTestChecker(newMockNamespaceValidator(ctrl), newMockOperations(ctrl))

	c.RequestCheck(nil, configWith(t, "storage1"))

	assert.Len(t, c.takePending().Storage, 1, "the request should be queued, not dropped")
}

// Start drains whatever accumulated before it, so the startup check is not lost.
func TestStart_DrainsRequestsMadeBeforeIt(t *testing.T) {
	ctrl := gomock.NewController(t)
	clusters := aerospike.NewMockNamespaceValidator(ctrl)
	clusters.EXPECT().Validate(gomock.Any(), gomock.Any()).AnyTimes()

	probed := make(chan model.Storage, 1)
	c := newTestChecker(clusters, recordingOperations(t, probed))

	c.RequestCheck(nil, configWith(t, "storage1"))

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	c.Start(ctx)

	select {
	case <-probed:
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not serve the check queued before it")
	}
}

// A change that reaches nothing new must not wake the worker at all.
func TestRequestCheck_EmptyDelta_QueuesNothing(t *testing.T) {
	ctrl := gomock.NewController(t)
	c := newTestChecker(newMockNamespaceValidator(ctrl), newMockOperations(ctrl))

	unchanged := configWith(t, "storage1")
	c.RequestCheck(unchanged, unchanged)

	assert.Empty(t, c.takePending().Storage)
	assert.Empty(t, c.signal, "an empty delta should not signal the worker")
}

// Requests made back to back merge by name, so a storage edited twice before the worker
// gets to it is probed once, with the definition that won.
func TestRequestCheck_CoalescesByName(t *testing.T) {
	ctrl := gomock.NewController(t)
	c := newTestChecker(newMockNamespaceValidator(ctrl), newMockOperations(ctrl))

	first := configWith(t, "storage1")
	second := configWith(t, "storage1")
	second.Storage["storage1"] = &model.LocalStorage{Path: "/tmp/second"}

	c.RequestCheck(nil, first)
	c.RequestCheck(first, second)

	pending := c.takePending()
	require.Len(t, pending.Storage, 1)
	assert.Equal(t, "/tmp/second", pending.Storage["storage1"].GetPath(), "the newer definition should win")
	assert.Len(t, c.signal, 1, "repeated requests should collapse into one wake-up")
}

// takePending hands the queue over and leaves an empty one, so a second pass does not
// re-probe what the first already covered.
func TestTakePending_LeavesTheQueueEmpty(t *testing.T) {
	ctrl := gomock.NewController(t)
	c := newTestChecker(newMockNamespaceValidator(ctrl), newMockOperations(ctrl))

	c.RequestCheck(nil, configWith(t, "storage1"))

	require.Len(t, c.takePending().Storage, 1)
	assert.Empty(t, c.takePending().Storage)
}

// recordingOperations reports every storage it was asked to probe on ch.
func recordingOperations(t *testing.T, ch chan<- model.Storage) storage.Operations {
	t.Helper()

	ops := storage.NewMockOperations(gomock.NewController(t))
	ops.EXPECT().
		Probe(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, s model.Storage) error {
			ch <- s

			return nil
		}).
		AnyTimes()

	return ops
}

func newMockOperations(ctrl *gomock.Controller) storage.Operations {
	return storage.NewMockOperations(ctrl)
}

func newMockNamespaceValidator(ctrl *gomock.Controller) aerospike.NamespaceValidator {
	return aerospike.NewMockNamespaceValidator(ctrl)
}

func configWith(t *testing.T, names ...string) *model.BackupConfig {
	t.Helper()

	config := model.NewConfig()
	for _, name := range names {
		require.NoError(t, config.AddStorage(name, &model.LocalStorage{Path: "/tmp/" + name}))
	}

	return config.BackupConfigCopy()
}
