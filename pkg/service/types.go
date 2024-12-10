package service

import (
	"context"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/backup-go"
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

type RestoreHandlerWithCancel struct {
	RestoreHandler
	cancel func()
}

func (rj *RestoreHandlerWithCancel) Cancel() {
	rj.cancel()
}

func (rj *RestoreHandlerWithCancel) GetStats() *models.RestoreStats {
	return rj.RestoreHandler.GetStats()
}

// Restore represents a restore service.
type Restore interface {
	Run(
		ctx context.Context,
		client *backup.Client,
		restoreRequest *model.RestoreRequest,
	) (RestoreHandler, error)
}
