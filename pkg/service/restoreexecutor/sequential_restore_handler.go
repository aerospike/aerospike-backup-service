package restoreexecutor

import (
	"context"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
)

// sequentialRestoreHandler follows restores that run one after another: it reports the counters
// of those started so far together, and its Wait ends when the last one ends or one fails.
type sequentialRestoreHandler struct {
	mu       sync.Mutex
	handlers []service.RestoreHandler
	// cancel stops the sequence.
	cancel context.CancelFunc
	done   chan struct{}
	err    error
}

var _ service.RestoreHandler = (*sequentialRestoreHandler)(nil)

func newSequentialRestoreHandler(cancel context.CancelFunc) *sequentialRestoreHandler {
	return &sequentialRestoreHandler{cancel: cancel, done: make(chan struct{})}
}

// add records a restore that has started.
func (h *sequentialRestoreHandler) add(handler service.RestoreHandler) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.handlers = append(h.handlers, handler)
}

// finish records the outcome of the whole sequence.
func (h *sequentialRestoreHandler) finish(err error) {
	h.err = err
	close(h.done)
}

// Wait waits for every restore of the sequence to finish, or for the first one to fail. If ctx
// ends first, Wait stops the sequence and returns once it has stopped, as backup-go's handlers
// do: the caller may then release what the restore reads from, such as the client and the
// backup files.
func (h *sequentialRestoreHandler) Wait(ctx context.Context) error {
	select {
	case <-h.done:
	case <-ctx.Done():
		h.cancel()
		<-h.done
	}

	return h.err
}

// Status returns the counters of every restore started so far, added up, with the metrics of the
// latest one: the rates of finished restores would not add up to anything.
func (h *sequentialRestoreHandler) Status() model.RestoreRun {
	h.mu.Lock()
	defer h.mu.Unlock()

	var run model.RestoreRun
	for _, handler := range h.handlers {
		status := handler.Status()
		run.Counters = run.Counters.Plus(status.Counters)
		run.Metrics = status.Metrics
	}

	return run
}
