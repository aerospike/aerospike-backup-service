package serverbackup

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/backup-go/models"
)

type namespaceRunner struct {
	clientManager aerospike.ClientManager
	resolver      CredentialsResolver
}

// NewNamespaceRunner builds a per-namespace server backup runner.
func NewNamespaceRunner(
	clientManager aerospike.ClientManager,
	resolver CredentialsResolver,
) *namespaceRunner {
	return &namespaceRunner{
		clientManager: clientManager,
		resolver:      resolver,
	}
}

// Run starts a server-side backup for one namespace.
func (e *namespaceRunner) Run(
	ctx context.Context,
	routine *model.BackupRoutine,
	namespace string,
	runSpec model.BackupRunSpec,
	logger *slog.Logger,
) service.CancelableBackupHandler {
	return newDirectBackupHandler(ctx, func(ctx context.Context) (backupHandler, error) {
		client, err := e.clientManager.GetClient(ctx, routine.SourceCluster, nil, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to get backup client: %w", err)
		}

		credentials, err := ResolveCredentials(ctx, e.resolver, routine.Storage)
		if err != nil {
			e.clientManager.Close(client)
			return nil, fmt.Errorf("failed to resolve server backup credentials: %w", err)
		}

		handler, err := Run(ctx, client.InfoClient(), namespace, routine, credentials, runSpec)
		if err != nil {
			e.clientManager.Close(client)
			return nil, err
		}

		return newCloseOnWaitHandler(handler, client, e.clientManager), nil
	})
}

type closeOnWaitHandler struct {
	inner         backupHandler
	client        aerospike.Client
	clientManager aerospike.ClientManager
	closeOnce     sync.Once
}

func newCloseOnWaitHandler(
	handler backupHandler,
	client aerospike.Client,
	clientManager aerospike.ClientManager,
) *closeOnWaitHandler {
	return &closeOnWaitHandler{
		inner:         handler,
		client:        client,
		clientManager: clientManager,
	}
}

func (h *closeOnWaitHandler) Wait(ctx context.Context) error {
	defer h.closeOnce.Do(func() { h.clientManager.Close(h.client) })
	return h.inner.Wait(ctx)
}

func (h *closeOnWaitHandler) GetMetrics() *models.Metrics {
	return h.inner.GetMetrics()
}

func (h *closeOnWaitHandler) GetStats() *models.BackupStats {
	return h.inner.GetStats()
}

type directBackupHandler struct {
	sync.RWMutex
	handler backupHandler
	cancel  context.CancelFunc
	errCh   chan error
}

var _ service.CancelableBackupHandler = (*directBackupHandler)(nil)

func newDirectBackupHandler(
	ctx context.Context,
	start func(context.Context) (backupHandler, error),
) *directBackupHandler {
	ctxWithCancel, cancel := context.WithCancel(ctx)
	h := &directBackupHandler{
		errCh:  make(chan error, 1),
		cancel: cancel,
	}

	go func() {
		handler, err := start(ctxWithCancel)
		if err != nil {
			h.errCh <- err
			return
		}

		h.setHandler(handler)
		h.errCh <- handler.Wait(ctxWithCancel)
	}()

	return h
}

func (h *directBackupHandler) setHandler(handler backupHandler) {
	h.Lock()
	defer h.Unlock()
	h.handler = handler
}

func (h *directBackupHandler) Wait(ctx context.Context) error {
	select {
	case err := <-h.errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *directBackupHandler) GetStats() *models.BackupStats {
	h.RLock()
	defer h.RUnlock()
	if h.handler != nil {
		return h.handler.GetStats()
	}

	return nil
}

func (h *directBackupHandler) GetMetrics() *models.Metrics {
	h.RLock()
	defer h.RUnlock()
	if h.handler != nil {
		return h.handler.GetMetrics()
	}

	return nil
}

func (h *directBackupHandler) Cancel() {
	h.cancel()
}
