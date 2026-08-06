package serverbackup

import (
	"errors"
	"fmt"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

var errBackupSkipped = errors.New("backup skipped")

// StartController allows at most one server backup per source cluster at a time.
type StartController struct {
	gate Gate
}

// NewStartController builds a cluster-gated start controller for server backup mode.
func NewStartController(gate Gate) *StartController {
	return &StartController{gate: gate}
}

// TryStart acquires the cluster gate or skips when a backup is already running.
func (c *StartController) TryStart(
	routine *model.BackupRoutine,
	_ time.Time,
	_ model.BackupType,
) (func(), error) {
	release, err := c.gate.TryAcquire(routine.SourceCluster.Hash())
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errBackupSkipped, err)
	}

	return release, nil
}

// HasBackupRunning reports whether a server backup is active on the routine's source cluster.
func (c *StartController) HasBackupRunning(routine *model.BackupRoutine) bool {
	return c.gate.IsActive(routine.SourceCluster.Hash())
}
