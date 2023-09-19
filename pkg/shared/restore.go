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

	"github.com/aerospike/backup/pkg/model"
)

type Restore interface {
	RestoreRun(restoreRequest *model.RestoreRequest)
}

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
func (r *RestoreShared) RestoreRun(restoreRequest *model.RestoreRequest) {
	// lock to restrict parallel execution (shared library limitation)
	r.Lock()
	defer r.Unlock()

	restoreConfig := C.restore_config_t{}
	C.restore_config_default(&restoreConfig)

	setCString(&restoreConfig.host, restoreRequest.Host)
	setCInt(&restoreConfig.port, restoreRequest.Port)

	setCString(&restoreConfig.user, restoreRequest.User)
	setCString(&restoreConfig.password, restoreRequest.Password)

	if len(restoreRequest.NsList) > 0 {
		nsList := strings.Join(restoreRequest.NsList, ",")
		setCString(&restoreConfig.ns_list, &nsList)
	}
	if len(restoreRequest.SetList) > 0 {
		setList := strings.Join(restoreRequest.SetList, ",")
		setCString(&restoreConfig.set_list, &setList)
	}
	if len(restoreRequest.BinList) > 0 {
		binList := strings.Join(restoreRequest.BinList, ",")
		setCString(&restoreConfig.bin_list, &binList)
	}

	// S3 configuration
	setCString(&restoreConfig.s3_endpoint_override, restoreRequest.S3EndpointOverride)
	setCString(&restoreConfig.s3_region, restoreRequest.S3Region)
	setCString(&restoreConfig.s3_profile, restoreRequest.S3Profile)
	setCString(&restoreConfig.directory, restoreRequest.Directory)

	setCBool(&restoreConfig.replace, restoreRequest.Replace)
	setCBool(&restoreConfig.unique, restoreRequest.Unique)
	setCBool(&restoreConfig.no_generation, restoreRequest.NoGeneration)

	// fmt.Println(restoreConfig)
	C.restore_run(&restoreConfig)

	// destroy the restore_config
	C.restore_config_destroy(&restoreConfig)
}
