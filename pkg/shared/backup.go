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
	"strings"
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
//
//nolint:funlen
func (b *BackupShared) BackupRun(backupPolicy *model.BackupPolicy, cluster *model.AerospikeCluster,
	storage *model.BackupStorage, opts BackupOptions) bool {
	// lock to restrict parallel execution (shared library limitation)
	b.Lock()
	defer b.Unlock()

	slog.Debug(fmt.Sprintf("Starting backup for %s", *backupPolicy.Name))

	backupConfig := C.backup_config_t{}
	C.backup_config_default(&backupConfig)

	setCString(&backupConfig.host, cluster.Host)
	setCInt(&backupConfig.port, cluster.Port)
	setCBool(&backupConfig.use_services_alternate, cluster.UseServicesAlternate)

	setCString(&backupConfig.user, cluster.User)
	setCString(&backupConfig.password, cluster.GetPassword())
	setCString(&backupConfig.auth_mode, cluster.AuthMode)

	parseSetList(&backupConfig.set_list, backupPolicy.SetList)
	setCString(&backupConfig.bin_list, backupPolicy.BinList)

	setCUint(&backupConfig.socket_timeout, backupPolicy.SocketTimeout)
	setCUint(&backupConfig.total_timeout, backupPolicy.TotalTimeout)
	setCUint(&backupConfig.max_retries, backupPolicy.MaxRetries)
	setCUint(&backupConfig.retry_delay, backupPolicy.RetryDelay)

	// namespace list configuration
	nsCharArray := C.CString(*backupPolicy.Namespace)
	C.strcpy((*C.char)(unsafe.Pointer(&backupConfig.ns)), nsCharArray)

	setCInt(&backupConfig.parallel, backupPolicy.Parallel)

	setCBool(&backupConfig.remove_files, backupPolicy.RemoveFiles)
	setCBool(&backupConfig.no_bins, backupPolicy.NoBins)
	setCBool(&backupConfig.no_records, backupPolicy.NoRecords)
	setCBool(&backupConfig.no_indexes, backupPolicy.NoIndexes)
	setCBool(&backupConfig.no_udfs, backupPolicy.NoUdfs)

	setCUlong(&backupConfig.bandwidth, backupPolicy.Bandwidth)
	setCUlong(&backupConfig.max_records, backupPolicy.MaxRecords)
	setCUint(&backupConfig.records_per_second, backupPolicy.RecordsPerSecond)
	setCUlong(&backupConfig.file_limit, backupPolicy.FileLimit)
	setCString(&backupConfig.partition_list, backupPolicy.PartitionList)
	setCString(&backupConfig.after_digest, backupPolicy.AfterDigest)
	setCString(&backupConfig.filter_exp, backupPolicy.FilterExp)

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

	backupStatus := C.backup_run(&backupConfig)

	var success bool
	if unsafe.Pointer(backupStatus) == C.RUN_BACKUP_SUCCESS { //nolint:gocritic
		success = true
	} else if unsafe.Pointer(backupStatus) != C.RUN_BACKUP_FAILURE {
		C.backup_status_destroy(backupStatus)
		C.cf_free(unsafe.Pointer(backupStatus))
		success = true
	} else {
		slog.Warn("Failed backup operation", "policy", backupPolicy.Name)
	}

	// destroy the backup_config
	C.backup_config_destroy(&backupConfig)
	return success
}

// parseSetList parses the configured set list for backup
func parseSetList(setVector *C.as_vector, setList *[]string) {
	if setList != nil && len(*setList) > 0 {
		concatenatedSetList := strings.Join(*setList, ",")
		C.parse_set_list(setVector, C.CString(concatenatedSetList))
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
