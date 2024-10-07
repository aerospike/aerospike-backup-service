package service

import (
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
)

const (
	metadataFile = "metadata.yaml"
	configExt    = ".conf"
)

func getFullPath(fullBackupsPath string, backupPolicy *model.BackupPolicy, namespace string,
	now time.Time) string {
	if backupPolicy.RemoveFiles.RemoveFullBackup() {
		return fmt.Sprintf("%s/%s/%s", fullBackupsPath, model.DataDirectory, namespace)
	}

	return fmt.Sprintf("%s/%s/%s/%s", fullBackupsPath, formatTime(now), model.DataDirectory, namespace)
}

func getIncrementalPath(incrBackupsPath string, t time.Time) string {
	return fmt.Sprintf("%s/%s", incrBackupsPath, formatTime(t))
}

func getIncrementalPathForNamespace(incrBackupsPath string, namespace string, t time.Time) string {
	return fmt.Sprintf("%s/%s/%s", getIncrementalPath(incrBackupsPath, t), model.DataDirectory, namespace)
}

func formatTime(t time.Time) string {
	return strconv.FormatInt(t.UnixMilli(), 10)
}

func getConfigurationFile(h *BackupRoutineHandler, t time.Time, i int) string {
	path := ""
	if h.backupFullPolicy.RemoveFiles.RemoveFullBackup() {
		path = fmt.Sprintf("%s/%s", h.backend.fullBackupsPath, model.ConfigurationBackupDirectory)
	} else {
		path = fmt.Sprintf("%s/%s/%s", h.backend.fullBackupsPath, formatTime(t), model.ConfigurationBackupDirectory)
	}

	return filepath.Join(path, getConfigFileName(i))
}

func getConfigFileName(i int) string {
	return fmt.Sprintf("aerospike_%d%s", i, configExt)
}

func getKey(path string, metadata *model.BackupMetadata, noTimestampInPath bool) string {
	if noTimestampInPath {
		return fmt.Sprintf("%s/data/%s", path, metadata.Namespace)
	}

	return fmt.Sprintf("%s/%d/data/%s", path, metadata.Created.UnixMilli(), metadata.Namespace)
}
