package tui

import (
	"context"
	"log/slog"
	"slices"
	"time"

	m "github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/reugn/go-quartz/quartz"
)

type Api interface {
	// read methods
	Config() *m.Config                        // return full read only config (once per session)
	Backups(routine string) []m.BackupDetails // return backups for a given routine (might be updated)

	RunningBackup(routine string) *m.RunningJob // state of a running backup (nil if not running)
	//	Restores(routine string) []model.RestoreJobStatus // state of a running restore (nil if not running)

	// actions
	StartBackup(routine string)
	CancelBackup(routine string)
	StartRestore(request m.RestoreTimestampRequest)
}

var _ Api = (*ApiImpl)(nil)

type ApiImpl struct {
	cfg            *m.Config
	backupReader   service.BackupReader
	registry       service.RunningBackupsRegistry
	scheduler      service.Scheduler
	restoreManager service.RestoreManager
}

func (a ApiImpl) CancelBackup(routine string) {
	a.registry.Cancel(routine)
}

func (a ApiImpl) RunningBackup(routine string) *m.RunningJob {
	state := a.registry.GetRoutineState(routine)
	return state.Full
}

func NewApiImpl(cfg *m.Config, backupReader service.BackupReader, registry service.RunningBackupsRegistry, scheduler service.Scheduler, restoreManager service.RestoreManager) Api {
	return &ApiImpl{
		cfg:            cfg,
		backupReader:   backupReader,
		registry:       registry,
		scheduler:      scheduler,
		restoreManager: restoreManager,
	}
}

func (a ApiImpl) Config() *m.Config {
	return a.cfg
}

func (a ApiImpl) Backups(routine string) []m.BackupDetails {
	backups, err := a.backupReader.GetBackups(context.TODO(), service.NewFullBackupFilter(routine))
	if err != nil {
		panic(err)
	}

	backupsIncr, err := a.backupReader.GetBackups(context.TODO(), service.NewIncrementalBackupFilter(routine))
	if err != nil {
		panic(err)
	}

	details := append(backups, backupsIncr...)

	slices.SortFunc(details, func(a, b m.BackupDetails) int {
		return -a.Created.Compare(b.Created)
	})

	return details
}

func (a ApiImpl) StartBackup(routine string) {
	job := service.NewAdHocFullBackupJobForRoutine(routine)
	if job == nil {
		slog.Error("Failed to create ad-hoc backup job", "routine", routine)
		return
	}
	trigger := quartz.NewRunOnceTrigger(time.Second)
	err := a.scheduler.ScheduleJob(job, trigger)
	if err != nil {
		slog.Error("Failed to schedule ad-hoc backup job", "routine", routine, "err", err)
	}
}

func (a ApiImpl) StartRestore(request m.RestoreTimestampRequest) {
	_, err := a.restoreManager.RestoreByTime(context.Background(), &request)
	if err != nil {
		slog.Error("Failed to start restore", "err", err)
	}
}
