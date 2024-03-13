package service

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/shared"
	"github.com/aerospike/backup/pkg/stdio"
	"github.com/aerospike/backup/pkg/util"
)

// BackupHandler implements backup logic for single routine.
type BackupHandler struct {
	backend          *BackupBackend
	backupFullPolicy *model.BackupPolicy
	backupIncrPolicy *model.BackupPolicy
	backupRoutine    *model.BackupRoutine
	routineName      string
	cluster          *model.AerospikeCluster
	storage          *model.Storage
	secretAgent      *model.SecretAgent
	state            *model.BackupState
	retry            *RetryService
}

var backupService shared.Backup = shared.NewBackup()

// newBackupHandler returns a new BackupHandler instance.
func newBackupHandler(config *model.Config, routineName string, backupBackend *BackupBackend) (*BackupHandler, error) {
	backupRoutine := config.BackupRoutines[routineName]
	cluster := config.AerospikeClusters[backupRoutine.SourceCluster]
	storage := config.Storage[backupRoutine.Storage]
	backupPolicy := config.BackupPolicies[backupRoutine.BackupPolicy]
	var secretAgent *model.SecretAgent
	if backupRoutine.SecretAgent != nil {
		secretAgent = config.SecretAgents[*backupRoutine.SecretAgent]
	}

	if len(backupRoutine.Namespaces) == 0 {
		namespaces, err := getAllNamespacesOfCluster(cluster)
		if err != nil {
			return nil, fmt.Errorf("failed to get namespaces: %v", err)
		}
		backupRoutine.Namespaces = namespaces
	}

	return &BackupHandler{
		backend:          backupBackend,
		backupRoutine:    backupRoutine,
		backupFullPolicy: backupPolicy,
		backupIncrPolicy: backupPolicy.CopySMDDisabled(), // incremental backups should not contain metadata
		cluster:          cluster,
		storage:          storage,
		secretAgent:      secretAgent,
		state:            backupBackend.readState(),
		routineName:      routineName,
		retry:            NewRetryService(routineName),
	}, nil
}

func (h *BackupHandler) runFullBackup(now time.Time) {
	h.retry.retry(
		func() error { return h.runFullBackupInternal(now) },
		time.Duration(*h.backupFullPolicy.RetryDelay)*time.Millisecond,
		*h.backupFullPolicy.MaxRetries,
	)
}

func (h *BackupHandler) runFullBackupInternal(now time.Time) error {
	if !h.backend.FullBackupInProgress().CompareAndSwap(false, true) {
		slog.Info("Full backup is currently in progress, skipping full backup", "name", h.routineName)
		return nil
	}
	slog.Debug("Acquire fullBackupInProgress lock", "name", h.routineName)
	// release the lock
	defer func() {
		h.backend.FullBackupInProgress().Store(false)
		slog.Debug("Release fullBackupInProgress lock", "name", h.routineName)
	}()
	for _, namespace := range h.backupRoutine.Namespaces {
		err := h.fullBackupForNamespace(now, namespace)
		if err != nil {
			return err
		}
	}

	// increment backupCounter metric
	backupCounter.Inc()

	// update the state
	h.updateFullBackupState(now)

	h.cleanIncrementalBackups()

	h.writeClusterConfiguration(now)
	return nil

}

func (h *BackupHandler) writeClusterConfiguration(now time.Time) {
	infos := GetInfo(h.cluster)
	if len(infos) == 0 {
		slog.Warn("Could not read aerospike configuration")
		return
	}
	path := getConfigurationPath(h.backend.confBackupPath, now)
	h.backend.CreateFolder(path)
	for i, info := range infos {
		confFilePath := fmt.Sprintf("%s/aerospike_%d.conf", path, i)
		slog.Info("Write aerospike configuration", "path", confFilePath)
		err := h.backend.write(confFilePath, []byte(info))
		if err != nil {
			slog.Error("Failed to write configuration for the backup", "name", h.routineName, "err", err)
		}
	}
}

func (h *BackupHandler) fullBackupForNamespace(upperBound time.Time, namespace string) error {
	backupFolder := getFullPath(h.backend.fullBackupsPath, h.backupFullPolicy, namespace, upperBound)
	h.backend.CreateFolder(backupFolder)

	options := shared.BackupOptions{}
	if h.backupFullPolicy.Sealed {
		options.ModBefore = util.Ptr(upperBound.UnixNano())
	}

	var stats *shared.BackupStat
	var err error
	backupRunFunc := func() {
		started := time.Now()
		backupPath := h.backend.wrapWithPrefix(backupFolder)
		stats, err = backupService.BackupRun(h.backupRoutine, h.backupFullPolicy, h.cluster,
			h.storage, h.secretAgent, options, &namespace, backupPath)
		if err != nil {
			return
		}
		elapsed := time.Since(started)
		backupDurationGauge.Set(float64(elapsed.Milliseconds()))
	}
	slog.Debug("Starting full backup", "up to", upperBound, "name", h.routineName)
	out := stdio.Stderr.Capture(backupRunFunc)
	slog.Debug("Completed full backup", "name", h.routineName)
	util.LogCaptured(out)

	if err != nil {
		backupFailureCounter.Inc()
		return fmt.Errorf("error during backup namespace %s, routine %s: %w", namespace, h.routineName, err)
	}

	metadata := stats.ToMetadata(time.Time{}, upperBound, namespace)
	if err := h.backend.writeBackupMetadata(backupFolder, metadata); err != nil {
		slog.Error("Could not write backup metadata", "name", h.routineName,
			"folder", backupFolder, "err", err)
		return err
	}
	return nil
}

func (h *BackupHandler) cleanIncrementalBackups() {
	if h.backupIncrPolicy.RemoveFiles.RemoveIncrementalBackup() {
		if err := h.backend.DeleteFolder(h.backend.incrementalBackupsPath); err != nil {
			slog.Error("Could not clean incremental backups", "name", h.routineName, "err", err)
		} else {
			slog.Info("Cleaned incremental backups", "name", h.routineName)
		}
	}
}

func (h *BackupHandler) runIncrementalBackup(now time.Time) {
	if h.state.LastFullRunIsEmpty() {
		slog.Log(context.Background(), util.LevelTrace,
			"Skip incremental backup until initial full backup is done",
			"name", h.routineName)
		return
	}
	if h.backend.FullBackupInProgress().Load() {
		slog.Log(context.Background(), util.LevelTrace,
			"Full backup is currently in progress, skipping incremental backup",
			"name", h.routineName)
		return
	}
	for _, namespace := range h.backupRoutine.Namespaces {
		h.runIncrBackupForNamespace(now, namespace)
	}

	// increment incrBackupCounter metric
	incrBackupCounter.Inc()

	// update the state
	h.updateIncrementalBackupState(now)
}

func (h *BackupHandler) runIncrBackupForNamespace(upperBound time.Time, namespace string) {
	backupFolder := getIncrementalPath(h.backend.incrementalBackupsPath, namespace, upperBound)
	h.backend.CreateFolder(backupFolder)

	var stats *shared.BackupStat
	var err error
	fromEpoch := h.state.LastRunEpoch()
	options := shared.BackupOptions{
		ModAfter: util.Ptr(fromEpoch),
	}
	if h.backupIncrPolicy.Sealed {
		options.ModBefore = util.Ptr(upperBound.UnixNano())
	}
	backupRunFunc := func() {
		started := time.Now()
		backupPath := h.backend.wrapWithPrefix(backupFolder)
		stats, err = backupService.BackupRun(
			h.backupRoutine, h.backupIncrPolicy, h.cluster, h.storage, h.secretAgent, options, &namespace, backupPath)
		if err != nil {
			slog.Warn("Failed incremental backup", "name", h.routineName)
			incrBackupFailureCounter.Inc()
			return
		}
		elapsed := time.Since(started)
		incrBackupDurationGauge.Set(float64(elapsed.Milliseconds()))
	}
	slog.Debug("Starting incremental backup", "name", h.routineName)
	out := stdio.Stderr.Capture(backupRunFunc)
	slog.Debug("Completed incremental backup", "name", h.routineName)
	util.LogCaptured(out)
	// delete if the backup file is empty
	if h.isBackupEmpty(stats) {
		h.deleteEmptyBackup(backupFolder, h.routineName)
	} else {
		metadata := stats.ToMetadata(time.Unix(0, fromEpoch), upperBound, namespace)
		if err := h.backend.writeBackupMetadata(backupFolder, metadata); err != nil {
			slog.Error("Could not write backup metadata", "name", h.routineName,
				"folder", backupFolder, "err", err)
		}
	}
}

func (h *BackupHandler) isBackupEmpty(stats *shared.BackupStat) bool {
	if stats == nil {
		return true
	}
	return stats.IsEmpty()
}

func (h *BackupHandler) deleteEmptyBackup(path string, routineName string) {
	if err := h.backend.DeleteFolder(path); err != nil {
		slog.Error("Failed to delete empty backup", "name", routineName,
			"path", path, "err", err)
	} else {
		slog.Debug("Deleted empty backup", "name", routineName, "path", path)
	}
}

func (h *BackupHandler) updateFullBackupState(now time.Time) {
	h.state.SetLastFullRun(now)
	h.writeState()
}

func (h *BackupHandler) updateIncrementalBackupState(now time.Time) {
	h.state.SetLastIncrRun(now)
	h.writeState()
}

func (h *BackupHandler) writeState() {
	if err := h.backend.writeState(h.state); err != nil {
		slog.Error("Failed to write state for the backup", "name", h.routineName, "err", err)
	}
}

func getFullPath(fullBackupsPath string, backupPolicy *model.BackupPolicy, namespace string, now time.Time) string {
	if backupPolicy.RemoveFiles.RemoveFullBackup() {
		path := fmt.Sprintf("%s/%s", fullBackupsPath, namespace)
		return path
	}
	path := fmt.Sprintf("%s/%s/%s", fullBackupsPath, timeSuffix(now), namespace)
	return path
}

func getIncrementalPath(incrBackupsPath string, namespace string, now time.Time) string {
	return fmt.Sprintf("%s/%s/%s", incrBackupsPath, timeSuffix(now), namespace)
}

func getConfigurationPath(confPath string, now time.Time) string {
	return fmt.Sprintf("%s/%s", confPath, timeSuffix(now))
}

func timeSuffix(now time.Time) string {
	return strconv.FormatInt(now.UnixMilli(), 10)
}
