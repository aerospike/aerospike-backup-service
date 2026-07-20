package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/prometheus"
	as "github.com/aerospike/aerospike-client-go/v8"
)

// BackupReporter logs backup outcomes and emits Prometheus metrics.
type BackupReporter interface {
	// Report logs the backup outcome and emits the corresponding Prometheus metrics.
	// For successful backups, err should be nil.
	// For skipped backups, err should be the skip reason.
	Report(
		routineName string,
		backupType model.BackupType,
		startTime time.Time,
		duration time.Duration,
		err error,
		logger *slog.Logger,
	)
}

type backupReporter struct{}

// NewBackupReporter returns a BackupReporter that logs and emits Prometheus metrics.
func NewBackupReporter() BackupReporter {
	return &backupReporter{}
}

var _ BackupReporter = (*backupReporter)(nil)

func (r *backupReporter) Report(
	routineName string,
	backupType model.BackupType,
	startTime time.Time,
	duration time.Duration,
	err error,
	logger *slog.Logger,
) {
	operation := string(backupType) + " backup"

	switch {
	case err == nil:
		logger.Debug(operation+" finished", slog.Duration("duration", duration))
		prometheus.ObserveBackupEvent(routineName, backupType, prometheus.OutcomeSuccess, duration, startTime)

	case errors.Is(err, errBackupSkipped):
		logger.Info(operation+" skipped", attr.Error(err))
		prometheus.ObserveBackupEvent(routineName, backupType, prometheus.OutcomeSkip, 0, startTime)

	case errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
		logger.Info(operation + " canceled")
		prometheus.ObserveBackupEvent(routineName, backupType, prometheus.OutcomeCanceled, duration, startTime)

	default:
		r.logFailure(operation, err, logger)
		prometheus.ObserveBackupEvent(routineName, backupType, prometheus.OutcomeFailure, duration, startTime)
	}
}

func (r *backupReporter) logFailure(operation string, err error, logger *slog.Logger) {
	var aerr *as.AerospikeError
	if errors.As(err, &aerr) {
		logger.Error(
			operation+" failed due to Aerospike error",
			slog.Int("resultCode", int(aerr.ResultCode)),
			attr.Error(err),
		)
	} else {
		logger.Error(operation+" failed", attr.Error(err))
	}
}
