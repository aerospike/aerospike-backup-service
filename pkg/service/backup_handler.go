package service

import (
	"context"
	"fmt"
	"github.com/aerospike/backup-go"
	"log/slog"
	"strconv"
	"time"

	"github.com/aerospike/aerospike-client-go/v7"
	"github.com/aerospike/backup-go/models"
	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/shared"
	"github.com/aerospike/backup/pkg/util"
)

// BackupHandler implements backup logic for single routine.
type BackupHandler struct {
	backend              *BackupBackend
	backupFullPolicy     *model.BackupPolicy
	backupIncrPolicy     *model.BackupPolicy
	backupRoutine        *model.BackupRoutine
	routineName          string
	namespaces           []string
	cluster              *model.AerospikeCluster
	storage              *model.Storage
	secretAgent          *model.SecretAgent
	state                *model.BackupState
	retry                *RetryService
	handlersForNamespace map[string]*backup.BackupHandler
}

var backupService shared.Backup = shared.NewBackupGo()

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

	namespaces := backupRoutine.Namespaces
	if len(namespaces) == 0 {
		var err error
		namespaces, err = getAllNamespacesOfCluster(cluster)
		if err != nil {
			return nil, fmt.Errorf("failed to get namespaces: %w", err)
		}
	}

	return &BackupHandler{
		backend:              backupBackend,
		backupRoutine:        backupRoutine,
		backupFullPolicy:     backupPolicy,
		backupIncrPolicy:     backupPolicy.CopySMDDisabled(), // incremental backups should not contain metadata
		routineName:          routineName,
		namespaces:           namespaces,
		cluster:              cluster,
		storage:              storage,
		secretAgent:          secretAgent,
		state:                backupBackend.readState(),
		retry:                NewRetryService(routineName),
		handlersForNamespace: make(map[string]*backup.BackupHandler),
	}, nil
}

func (h *BackupHandler) runFullBackup(now time.Time) {
	h.retry.retry(
		func() error { return h.runFullBackupInternal(now) },
		time.Duration(h.backupFullPolicy.GetRetryDelayOrDefault())*time.Millisecond,
		h.backupFullPolicy.GetMaxRetriesOrDefault(),
	)
}

func (h *BackupHandler) runFullBackupInternal(now time.Time) error {
	if !h.backend.FullBackupInProgress().CompareAndSwap(false, true) {
		slog.Info("Full backup is currently in progress, skipping full backup", "name", h.routineName)
		return nil
	}
	slog.Debug("Acquire fullBackupInProgress lock", "name", h.routineName)

	client, aerr := aerospike.NewClientWithPolicyAndHost(h.cluster.ASClientPolicy(), h.cluster.ASClientHosts()...)
	if aerr != nil {
		return fmt.Errorf("failed to connect to aerospike cluster, %w", aerr)
	}

	// release the lock
	defer func() {
		h.backend.FullBackupInProgress().Store(false)
		slog.Debug("Release fullBackupInProgress lock", "name", h.routineName)
		client.Close()
	}()

	startTime := time.Now()
	err := h.backupNamespaces(now, client)
	if err != nil {
		return err
	}
	backupDurationGauge.Set(float64(time.Since(startTime).Milliseconds()))

	// increment backupCounter metric
	backupCounter.Inc()

	// update the state
	h.updateFullBackupState(now)

	h.cleanIncrementalBackups()

	h.writeClusterConfiguration(now)
	return nil
}

func (h *BackupHandler) backupNamespaces(now time.Time, client *aerospike.Client) error {
	for _, namespace := range h.namespaces {
		backupFolder := getFullPath(h.backend.fullBackupsPath, h.backupFullPolicy, namespace, now)
		backupHandler, err := h.startBackup(client, backupFolder, now, namespace)
		if err != nil {
			return err
		}
		h.handlersForNamespace[namespace] = backupHandler
	}

	for namespace, handler := range h.handlersForNamespace {
		err := handler.Wait(context.TODO())
		if err != nil {
			backupFailureCounter.Inc()
			return fmt.Errorf("error during backup namespace %s, routine %s: %w", namespace, h.routineName, err)
		}

		backupFolder := getFullPath(h.backend.fullBackupsPath, h.backupFullPolicy, namespace, now)
		if err := h.writeBackupMetadata(handler.GetStats(), now, namespace, backupFolder); err != nil {
			return err
		}
	}

	return nil
}

func (h *BackupHandler) startBackup(client *aerospike.Client, backupFolder string, upperBound time.Time, namespace string) (*backup.BackupHandler, error) {
	h.backend.CreateFolder(backupFolder)
	options := shared.BackupOptions{}
	if h.backupFullPolicy.IsSealed() {
		options.ModBefore = &upperBound
	}

	backupPath := h.backend.wrapWithPrefix(backupFolder)
	handler, err := backupService.BackupRun(h.backupRoutine, h.backupFullPolicy, client,
		h.storage, h.secretAgent, options, &namespace, backupPath)
	if err != nil {
		backupFailureCounter.Inc()
		return nil, fmt.Errorf("could not start backup of namespace %s, routine %s: %w", namespace, h.routineName, err)
	}

	return handler, nil
}

func (h *BackupHandler) writeClusterConfiguration(now time.Time) {
	infos, err := getClusterConfiguration(h.cluster)
	if err != nil || len(infos) == 0 {
		slog.Warn("Could not read aerospike configuration", "err", err, "name", h.routineName)
		return
	}
	path := getConfigurationPath(h.backend.fullBackupsPath, h.backupFullPolicy, now)
	h.backend.CreateFolder(path)
	for i, info := range infos {
		confFilePath := fmt.Sprintf("%s/aerospike_%d.conf", path, i)
		slog.Debug("Write aerospike configuration", "path", confFilePath)
		err := h.backend.write(confFilePath, []byte(info))
		if err != nil {
			slog.Error("Failed to write configuration for the backup", "name", h.routineName, "err", err)
		}
	}
}

func (h *BackupHandler) writeBackupMetadata(stats *models.BackupStats,
	created time.Time,
	namespace string,
	backupFolder string) error {
	metadata := model.BackupMetadata{
		From:                time.Time{},
		Created:             created,
		Namespace:           namespace,
		RecordCount:         stats.GetRecordsReadTotal(),
		FileCount:           stats.GetFileCount(),
		ByteCount:           stats.GetTotalBytesWritten(),
		SecondaryIndexCount: uint64(stats.GetSIndexes()),
		UDFCount:            uint64(stats.GetUDFs()),
	}

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
		slog.Debug("Skip incremental backup until initial full backup is done",
			"name", h.routineName)
		return
	}
	if h.backend.FullBackupInProgress().Load() {
		slog.Debug("Full backup is currently in progress, skipping incremental backup",
			"name", h.routineName)
		return
	}

	client, err := aerospike.NewClientWithPolicyAndHost(h.cluster.ASClientPolicy(), h.cluster.ASClientHosts()...)
	if err != nil {
		slog.Error("failed to connect to aerospike cluster", "err", err)
	}

	for _, namespace := range h.namespaces {
		h.runIncrBackupForNamespace(client, now, namespace)
	}

	// increment incrBackupCounter metric
	incrBackupCounter.Inc()

	// update the state
	h.updateIncrementalBackupState(now)
}

func (h *BackupHandler) runIncrBackupForNamespace(client *aerospike.Client, upperBound time.Time, namespace string) {
	backupFolder := getIncrementalPath(h.backend.incrementalBackupsPath, namespace, upperBound)
	h.backend.CreateFolder(backupFolder)

	fromEpoch := h.state.LastRunEpoch()
	options := shared.BackupOptions{
		ModAfter: util.Ptr(time.Unix(0, fromEpoch)),
	}
	if h.backupIncrPolicy.IsSealed() {
		options.ModBefore = &upperBound
	}

	slog.Debug("Starting incremental backup", "name", h.routineName)
	started := time.Now()
	backupPath := h.backend.wrapWithPrefix(backupFolder)
	handler, err := backupService.BackupRun(
		h.backupRoutine, h.backupIncrPolicy, client, h.storage, h.secretAgent, options, &namespace, backupPath)
	if err != nil {
		incrBackupFailureCounter.Inc()
		slog.Warn("could not start backup", "namespace", namespace, "routine", h.routineName, "err", err)
	}

	incrBackupDurationGauge.Set(float64(time.Since(started).Milliseconds()))
	slog.Debug("Completed incremental backup", "name", h.routineName)

	err = handler.Wait(context.TODO())
	if err != nil {
		slog.Warn("Failed incremental backup", "name", h.routineName, "err", err)
		incrBackupFailureCounter.Inc()
		return
	}

	// delete if the backup file is empty
	if h.isBackupEmpty(handler.GetStats()) {
		h.deleteEmptyBackup(backupFolder, h.routineName)
	} else {
		if err := h.writeBackupMetadata(handler.GetStats(), upperBound, namespace, backupFolder); err != nil {
			slog.Error("Could not write backup metadata", "name", h.routineName,
				"folder", backupFolder, "err", err)
		}
	}
}

func (h *BackupHandler) isBackupEmpty(stats *models.BackupStats) bool {
	return stats.GetUDFs() == 0 &&
		stats.GetSIndexes() == 0 &&
		stats.GetRecordsReadTotal() == 0
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
		return fmt.Sprintf("%s/%s/%s", fullBackupsPath, model.DataDirectory, namespace)
	}
	return fmt.Sprintf("%s/%s/%s/%s", fullBackupsPath, timeSuffix(now), model.DataDirectory, namespace)
}

func getIncrementalPath(incrBackupsPath string, namespace string, now time.Time) string {
	return fmt.Sprintf("%s/%s/%s/%s", incrBackupsPath, timeSuffix(now), model.DataDirectory, namespace)
}

func getConfigurationPath(fullBackupsPath string, backupPolicy *model.BackupPolicy, now time.Time) string {
	if backupPolicy.RemoveFiles.RemoveFullBackup() {
		path := fmt.Sprintf("%s/%s", fullBackupsPath, model.ConfigurationBackupDirectory)
		return path
	}

	return fmt.Sprintf("%s/%s/%s", fullBackupsPath, timeSuffix(now), model.ConfigurationBackupDirectory)
}

func timeSuffix(now time.Time) string {
	return strconv.FormatInt(now.UnixMilli(), 10)
}

func (h *BackupHandler) GetCurrentStat() int {
	var total, done uint64
	for _, handler := range h.handlersForNamespace {
		done += handler.GetStats().GetRecordsReadTotal()
		total += handler.GetStats().RecordsToBackup
	}

	if total == 0 {
		return 0
	}

	// TODO: add more data
	return int(100 * done / total)
}
