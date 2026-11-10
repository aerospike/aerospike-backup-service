package serverbackup

import (
	"fmt"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
)

var errClusterBusy = fmt.Errorf("%w: a server backup is already running on the cluster", service.ErrBackupSkipped)

// startController adds a per-cluster gate to the routine's own start rules: the server runs a
// single backup job at a time, whatever routine asks for it.
type startController struct {
	routineGate service.StartController

	mu     sync.Mutex
	active map[uint64]struct{}
}

var _ service.StartController = (*startController)(nil)

// NewStartController returns a StartController that admits a backup only when routineGate does
// and no other server backup holds the routine's source cluster.
func NewStartController(routineGate service.StartController) service.StartController {
	return &startController{
		routineGate: routineGate,
		active:      make(map[uint64]struct{}),
	}
}

// TryStart applies the routine's start rules, then reserves the routine's source cluster. The
// release frees both.
func (c *startController) TryStart(
	routine *model.BackupRoutine,
	now time.Time,
	backupType model.BackupType,
) (func(), error) {
	releaseRoutine, err := c.routineGate.TryStart(routine, now, backupType)
	if err != nil {
		return nil, err
	}

	key := routine.SourceCluster.Hash()

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, busy := c.active[key]; busy {
		releaseRoutine()
		return nil, errClusterBusy
	}
	c.active[key] = struct{}{}

	return sync.OnceFunc(func() {
		c.mu.Lock()
		delete(c.active, key)
		c.mu.Unlock()

		releaseRoutine()
	}), nil
}

// HasBackupRunning reports whether the routine itself runs a backup: the cluster gate holds no
// routine's backup but its own.
func (c *startController) HasBackupRunning(routine *model.BackupRoutine) bool {
	return c.routineGate.HasBackupRunning(routine)
}
