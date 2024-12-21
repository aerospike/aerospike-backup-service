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
		return filepath.Join(routineName, fullBackupDirectory)
	}
	return filepath.Join(routineName, incrementalBackupDirectory)
}

func getFullPath(routineName string, namespace string, timestamp time.Time) string {
	return filepath.Join(routineName, fullBackupDirectory, formatTimestamp(timestamp), dataDirectory, namespace)
}

func getIncrementalPath(routineName string, namespace string, timestamp time.Time) string {
	return filepath.Join(routineName, incrementalBackupDirectory, formatTimestamp(timestamp), dataDirectory, namespace)
}

func getConfigurationPath(routineName string, timestamp time.Time, index int) string {
	return filepath.Join(routineName, fullBackupDirectory, formatTimestamp(timestamp),
		configurationBackupDirectory, getConfigFileName(index))
}

func getKey(routineName string, isFullBackup bool, metadata *model.BackupMetadata) string {
	backupDir := fullBackupDirectory
	if !isFullBackup {
		backupDir = incrementalBackupDirectory
	}

	return filepath.Join(routineName, backupDir, formatTimestamp(metadata.Created), dataDirectory, metadata.Namespace)
}

func formatTimestamp(t time.Time) string {
	return strconv.FormatInt(t.UnixMilli(), 10)
}

func getConfigFileName(index int) string {
	return fmt.Sprintf("aerospike_%d%s", index, configExt)
}
