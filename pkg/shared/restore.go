//go:build !ci

package shared

/*
#cgo CFLAGS: -I../../modules/aerospike-tools-backup/include -I../../modules/aerospike-tools-backup/modules/c-client/target/Darwin-x86_64/include
#cgo LDFLAGS: -L${SRCDIR}/../../lib -lasrestore

#include <stddef.h>
#include <stdio.h>
#include <stdint.h>

#include <restore.h>
*/
import "C"
import (
	"strings"
	"sync"
	"unsafe"

	"log/slog"

	"github.com/aerospike/backup/pkg/model"
)

// RestoreShared implements the Restore interface.
type RestoreShared struct {
	sync.Mutex
}

var _ Restore = (*RestoreShared)(nil)

// NewRestore returns a new RestoreShared instance.
func NewRestore() *RestoreShared {
	return &RestoreShared{}
}

// RestoreRun calls the restore_run function from the asrestore shared library.
//
//nolint:funlen
func (r *RestoreShared) RestoreRun(restoreRequest *model.RestoreRequest) bool {
	// lock to restrict parallel execution (shared library limitation)
	r.Lock()
	defer r.Unlock()

	slog.Debug("Starting restore operation")

	restoreConfig := C.restore_config_t{}
	C.restore_config_default(&restoreConfig)

	setCString(&restoreConfig.host, restoreRequest.DestinationCuster.Host)
	setCInt(&restoreConfig.port, restoreRequest.DestinationCuster.Port)
	setCBool(&restoreConfig.use_services_alternate, restoreRequest.DestinationCuster.UseServicesAlternate)

	setCString(&restoreConfig.user, restoreRequest.DestinationCuster.User)
	setCString(&restoreConfig.password, restoreRequest.DestinationCuster.Password)
	setCString(&restoreConfig.auth_mode, restoreRequest.DestinationCuster.AuthMode)

	setCUint(&restoreConfig.parallel, restoreRequest.Policy.Parallel)
	setCBool(&restoreConfig.no_records, restoreRequest.Policy.NoRecords)
	setCBool(&restoreConfig.no_indexes, restoreRequest.Policy.NoIndexes)
	setCBool(&restoreConfig.no_udfs, restoreRequest.Policy.NoUdfs)

	setCUint(&restoreConfig.timeout, restoreRequest.Policy.Timeout)

	setCBool(&restoreConfig.disable_batch_writes, restoreRequest.Policy.DisableBatchWrites)
	setCUint(&restoreConfig.max_async_batches, restoreRequest.Policy.MaxAsyncBatches)
	setCUint(&restoreConfig.batch_size, restoreRequest.Policy.BatchSize)

	if len(restoreRequest.Policy.NsList) > 0 {
		nsList := strings.Join(restoreRequest.Policy.NsList, ",")
		setCString(&restoreConfig.ns_list, &nsList)
	}
	if len(restoreRequest.Policy.SetList) > 0 {
		setList := strings.Join(restoreRequest.Policy.SetList, ",")
		setCString(&restoreConfig.set_list, &setList)
	}
	if len(restoreRequest.Policy.BinList) > 0 {
		binList := strings.Join(restoreRequest.Policy.BinList, ",")
		setCString(&restoreConfig.bin_list, &binList)
	}

	// S3 configuration
	setCString(&restoreConfig.s3_endpoint_override, restoreRequest.SourceStorage.S3EndpointOverride)
	setCString(&restoreConfig.s3_region, restoreRequest.SourceStorage.S3Region)
	setCString(&restoreConfig.s3_profile, restoreRequest.SourceStorage.S3Profile)

	// restore source configuration
	setCString(&restoreConfig.directory, restoreRequest.Directory)
	setCString(&restoreConfig.input_file, restoreRequest.File)

	setCBool(&restoreConfig.replace, restoreRequest.Policy.Replace)
	setCBool(&restoreConfig.unique, restoreRequest.Policy.Unique)
	setCBool(&restoreConfig.no_generation, restoreRequest.Policy.NoGeneration)

	setCUlong(&restoreConfig.bandwidth, restoreRequest.Policy.Bandwidth)
	setCUint(&restoreConfig.tps, restoreRequest.Policy.Tps)

	restoreStatus := C.restore_run(&restoreConfig)

	var success bool
	if unsafe.Pointer(restoreStatus) != C.RUN_RESTORE_FAILURE {
		C.restore_status_destroy(restoreStatus)
		C.cf_free(unsafe.Pointer(restoreStatus))
		success = true
	} else {
		slog.Warn("Failed restore operation", "request", restoreRequest)
	}

	// destroy the restore_config
	C.restore_config_destroy(&restoreConfig)
	return success
}
