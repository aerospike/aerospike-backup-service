package service

import (
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
)

const (
	metadataFile                 = "metadata.yaml"
	configExt                    = ".conf"
	incrementalBackupDirectory   = "incremental"
	fullBackupDirectory          = "backup"
	configurationBackupDirectory = "configuration"
	dataDirectory                = "data"
)

func getBackupRootPath(routineName string, isFullBackup bool) string {
	if isFullBackup {
		return join(routineName, fullBackupDirectory)
	}
	return join(routineName, incrementalBackupDirectory)
}

func getIncrementalRoot(routineName string) string {
	return getBackupRootPath(routineName, false)
}

func getFullPath(routineName string, policy *model.BackupPolicy, namespace string, timestamp time.Time) string {
	if policy.RemoveFiles.RemoveFullBackup() {
		return join(routineName, fullBackupDirectory, dataDirectory, namespace)
	}
	return join(routineName, fullBackupDirectory, formatTimestamp(timestamp), dataDirectory, namespace)
}

func getIncrementalPathForNamespace(routineName string, namespace string, timestamp time.Time) string {
	return join(routineName, incrementalBackupDirectory, formatTimestamp(timestamp), dataDirectory, namespace)
}

func getIncrementalTimestampPath(routineName string, timestamp time.Time) string {
	return join(routineName, incrementalBackupDirectory, formatTimestamp(timestamp))
}

func getConfigurationPath(routineName string, policy *model.BackupPolicy, timestamp time.Time, index int) string {
	if policy.RemoveFiles.RemoveFullBackup() {
		return join(routineName, fullBackupDirectory, configurationBackupDirectory, getConfigFileName(index))
	}
	return join(routineName, fullBackupDirectory, formatTimestamp(timestamp), configurationBackupDirectory, getConfigFileName(index))
}

func getKey(routineName string, isFullBackup bool, metadata *model.BackupMetadata, noTimestampInPath bool) string {
	backupDir := fullBackupDirectory
	if !isFullBackup {
		backupDir = incrementalBackupDirectory
	}

	if noTimestampInPath {
		return join(routineName, backupDir, dataDirectory, metadata.Namespace)
	}
	return join(routineName, backupDir, formatTimestamp(metadata.Created), dataDirectory, metadata.Namespace)
}

func formatTimestamp(t time.Time) string {
	return strconv.FormatInt(t.UnixMilli(), 10)
}

func getConfigFileName(index int) string {
	return fmt.Sprintf("aerospike_%d%s", index, configExt)
}

func join(elements ...string) string {
	return filepath.Join(elements...)
}
