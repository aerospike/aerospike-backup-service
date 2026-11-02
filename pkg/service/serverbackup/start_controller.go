package serverbackup

import (
	"fmt"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
)

var errClusterBusy = fmt.Errorf("%w: a server backup is already running on the cluster", service.ErrBackupSkipped)

// startController admits at most one server backup per source cluster: the server runs a
// single backup job at a time, whatever routine asks for it.
type startController struct {
	mu     sync.Mutex
	active map[uint64]struct{}
}

var _ service.StartController = (*startController)(nil)

// NewStartController returns a StartController that gates server backups per source cluster.
func NewStartController() service.StartController {
	return &startController{
		active: make(map[uint64]struct{}),
	}
}

// TryStart reserves the routine's source cluster, or fails when a backup already holds it.
func (c *startController) TryStart(
	routine *model.BackupRoutine,
	_ time.Time,
	_ model.BackupType,
) (func(), error) {
	key := routine.SourceCluster.Hash()

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, busy := c.active[key]; busy {
		return nil, errClusterBusy
	}
	c.active[key] = struct{}{}

	return sync.OnceFunc(func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		delete(c.active, key)
	}), nil
}

// HasBackupRunning reports whether a server backup holds the routine's source cluster.
func (c *startController) HasBackupRunning(routine *model.BackupRoutine) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	_, busy := c.active[routine.SourceCluster.Hash()]

	return busy
}
