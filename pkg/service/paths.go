package service

import (
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

const (
	metadataFile                 = "metadata.yaml"
	configExt                    = ".conf"
	incrementalBackupDirectory   = "incremental"
	fullBackupDirectory          = "backup"
	configurationBackupDirectory = "configuration"
	dataDirectory                = "data"
)

func getBackupRootPath(routineName string, backupType jobType) string {
	if backupType == jobTypeFull {
		return filepath.Join(routineName, fullBackupDirectory)
	}

	return filepath.Join(routineName, incrementalBackupDirectory)
}

func getTimestampPath(routineName string, timestamp time.Time, backupType jobType) string {
	return filepath.Join(getBackupRootPath(routineName, backupType), formatTimestamp(timestamp))
}

func getBackupPath(routineName string, backupType jobType, namespace string, timestamp time.Time) string {
	if backupType == jobTypeFull {
		return filepath.Join(routineName, fullBackupDirectory, formatTimestamp(timestamp), dataDirectory, namespace)
	}

	return filepath.Join(routineName, incrementalBackupDirectory, formatTimestamp(timestamp), dataDirectory, namespace)
}

func getConfigurationPath(routineName string, timestamp time.Time, index int) string {
	return filepath.Join(routineName, fullBackupDirectory, formatTimestamp(timestamp),
		configurationBackupDirectory, getConfigFileName(index))
}

func getKey(routineName string, backupType jobType, metadata *model.BackupMetadata) string {
	rootPath := getBackupRootPath(routineName, backupType)
	return filepath.Join(rootPath, formatTimestamp(metadata.Created), dataDirectory, metadata.Namespace)
}

func formatTimestamp(t time.Time) string {
	return strconv.FormatInt(t.UnixMilli(), 10)
}

func getConfigFileName(index int) string {
	return fmt.Sprintf("aerospike_%d%s", index, configExt)
}
