package app

import (
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/restoreexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	u "github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
)

// restoreStack holds components built for the restore domain.
type restoreStack struct {
	RestoreManager    service.RestoreManager
	RestoreJobsHolder *service.RestoreJobsHolder
}

// newRestoreStack builds RestoreJobsHolder and RestoreManager.
func newRestoreStack(
	operations *storage.Operations,
	clientManager aerospike.ClientManager,
	backendService service.BackupReader,
	routineStorage *u.LockMap,
) restoreStack {
	restoreJobs := service.NewRestoreJobsHolder()
	restoreMgr := service.NewRestoreManager(
		restoreexecutor.NewRestore(operations),
		clientManager,
		restoreJobs,
		backendService,
		routineStorage,
	)
	return restoreStack{
		RestoreManager:    restoreMgr,
		RestoreJobsHolder: restoreJobs,
	}
}
