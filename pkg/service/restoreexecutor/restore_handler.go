package restoreexecutor

import (
	"context"

	"github.com/aerospike/backup-go/models"
)

// RestoreHandler represents a restore handler returned by the backup client.
type RestoreHandler interface {
	// GetStats returns the statistics of the restore job.
	GetStats() *models.RestoreStats
	// Wait waits for the restore job to complete and returns an error if the
	// job failed.
	Wait(context.Context) error
}

// RestoreHandlerWithCancel is a wrapper around a RestoreHandler that adds a cancel function.
type RestoreHandlerWithCancel struct {
	RestoreHandler
	cancel func()
}

// NewRestoreHandlerWithCancel creates a new RestoreHandlerWithCancel.
func NewRestoreHandlerWithCancel(handler RestoreHandler, cancelFunc func()) *RestoreHandlerWithCancel {
	return &RestoreHandlerWithCancel{
		RestoreHandler: handler,
		cancel:         cancelFunc,
	}
}

// Cancel cancels the restore job.
func (rj *RestoreHandlerWithCancel) Cancel() {
	rj.cancel()
}

// GetStats returns the statistics of the restore job.
func (rj *RestoreHandlerWithCancel) GetStats() *models.RestoreStats {
	return rj.RestoreHandler.GetStats()
}
