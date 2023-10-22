//go:build !ci

package shared

/*
#cgo CFLAGS: -I../../modules/aerospike-tools-backup/include -I../../modules/aerospike-tools-backup/modules/c-client/target/Darwin-x86_64/include
#cgo LDFLAGS: -L${SRCDIR}/../../lib -lasbackup

#include <stddef.h>
#include <stdio.h>
#include <stdint.h>

#include <backup.h>
#include <utils.h>
*/
import "C"
import (
	"fmt"
	"strconv"
	"sync"
	"time"
	"unsafe"

	"log/slog"

	"github.com/aerospike/backup/pkg/model"
)

// BackupShared implements the Backup interface.
type BackupShared struct {
	sync.Mutex
}

var _ Backup = (*BackupShared)(nil)

// NewBackup returns a new BackupShared instance.
func NewBackup() *BackupShared {
	return &BackupShared{}
}

// BackupRun calls the backup_run function from the asbackup shared library.
func (b *BackupShared) BackupRun(backupPolicy *model.BackupPolicy, cluster *model.AerospikeCluster,
	storage *model.BackupStorage, opts BackupOptions) {
	// lock to restrict parallel execution (shared library limitation)
	b.Lock()
	defer b.Unlock()

	slog.Debug(fmt.Sprintf("Starting backup for %s", *backupPolicy.Name))

	backupConfig := C.backup_config_t{}
	C.backup_config_default(&backupConfig)

	setCString(&backupConfig.host, cluster.Host)
	setCInt(&backupConfig.port, cluster.Port)

	setCString(&backupConfig.user, cluster.User)
	setCString(&backupConfig.password, cluster.Password)

	setVector(&backupConfig.set_list, backupPolicy.SetList)

	// namespace list configuration
	nsCharArray := C.CString(*backupPolicy.Namespace)
	C.strcpy((*C.char)(unsafe.Pointer(&backupConfig.ns)), nsCharArray)

	setCInt(&backupConfig.parallel, backupPolicy.Parallelism)

	setCBool(&backupConfig.remove_files, backupPolicy.RemoveFiles)
	setCBool(&backupConfig.no_bins, backupPolicy.NoBins)
	setCBool(&backupConfig.no_records, backupPolicy.NoRecords)
	setCBool(&backupConfig.no_indexes, backupPolicy.NoIndexes)
	setCBool(&backupConfig.no_udfs, backupPolicy.NoUdfs)

	// S3 configuration
	setCString(&backupConfig.s3_endpoint_override, storage.S3EndpointOverride)
	setCString(&backupConfig.s3_region, storage.S3Region)
	setCString(&backupConfig.s3_profile, storage.S3Profile)

	if opts.ModAfter != nil {
		// for incremental backup
		setCLong(&backupConfig.mod_after, opts.ModAfter)
		setCString(&backupConfig.output_file, getIncrementalPath(storage))
	} else {
		// for full backup
		setCString(&backupConfig.directory, getPath(storage, backupPolicy))
	}

	// fmt.Println(backupConfig)
	C.backup_run(&backupConfig)

	// destroy the backup_config
	C.backup_config_destroy(&backupConfig)
}

// set the as_vector for the backup_config
func setVector(setVector *C.as_vector, setList *[]string) {
	if setList != nil && len(*setList) > 0 {
		C.as_vector_init(setVector, 64, C.uint(len(*setList)))
		for i, setName := range *setList {
			setCharArray := unsafe.Pointer(C.CString(setName))
			C.as_vector_set(setVector, C.uint(i), setCharArray)
		}
	}
}

func getPath(storage *model.BackupStorage, backupPolicy *model.BackupPolicy) *string {
	if backupPolicy.RemoveFiles != nil && !*backupPolicy.RemoveFiles {
		path := fmt.Sprintf("%s/%s/%s", *storage.Path, model.FullBackupDirectory, timeSuffix())
		return &path
	}
	path := fmt.Sprintf("%s/%s", *storage.Path, model.FullBackupDirectory)
	return &path
}

func getIncrementalPath(storage *model.BackupStorage) *string {
	path := fmt.Sprintf("%s/%s/%s.asb", *storage.Path, model.IncrementalBackupDirectory, timeSuffix())
	return &path
}

func timeSuffix() string {
	return strconv.FormatInt(time.Now().Unix(), 10)
}
