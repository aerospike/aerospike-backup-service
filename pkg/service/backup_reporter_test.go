package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	metrics "github.com/aerospike/aerospike-backup-service/v3/pkg/service/prometheus"
	as "github.com/aerospike/aerospike-client-go/v8"
	astypes "github.com/aerospike/aerospike-client-go/v8/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestBackupReporter_Report_Success(t *testing.T) {
	reporter := NewBackupReporter()
	duration := 2 * time.Minute

	before := backupEventCount(t, "routine-success", model.BackupTypeFull, metrics.OutcomeSuccess)
	reporter.Report(
		"routine-success",
		model.BackupTypeFull,
		duration,
		nil,
		slog.New(slog.DiscardHandler),
	)
	require.Equal(t, before+1, backupEventCount(t, "routine-success", model.BackupTypeFull, metrics.OutcomeSuccess))
}

func TestBackupReporter_Report_Skipped(t *testing.T) {
	reporter := NewBackupReporter()

	before := backupEventCount(t, "routine-skip", model.BackupTypeIncremental, metrics.OutcomeSkip)
	reporter.Report(
		"routine-skip",
		model.BackupTypeIncremental,
		time.Minute,
		fmt.Errorf("wrapped: %w", ErrBackupSkipped),
		slog.New(slog.DiscardHandler),
	)
	require.Equal(t, before+1, backupEventCount(t, "routine-skip", model.BackupTypeIncremental, metrics.OutcomeSkip))
}

func TestBackupReporter_Report_Canceled(t *testing.T) {
	reporter := NewBackupReporter()

	before := backupEventCount(t, "routine-cancel", model.BackupTypeFull, metrics.OutcomeCanceled)
	reporter.Report(
		"routine-cancel",
		model.BackupTypeFull,
		30*time.Second,
		context.Canceled,
		slog.New(slog.DiscardHandler),
	)
	require.Equal(t, before+1, backupEventCount(t, "routine-cancel", model.BackupTypeFull, metrics.OutcomeCanceled))
}

func TestBackupReporter_Report_DeadlineExceeded(t *testing.T) {
	reporter := NewBackupReporter()

	before := backupEventCount(t, "routine-deadline", model.BackupTypeFull, metrics.OutcomeCanceled)
	reporter.Report(
		"routine-deadline",
		model.BackupTypeFull,
		30*time.Second,
		context.DeadlineExceeded,
		slog.New(slog.DiscardHandler),
	)
	require.Equal(t, before+1, backupEventCount(t, "routine-deadline", model.BackupTypeFull, metrics.OutcomeCanceled))
}

func TestBackupReporter_Report_GenericFailure(t *testing.T) {
	reporter := NewBackupReporter()
	logger, logBuf := newTestLogger(t)

	before := backupEventCount(t, "routine-fail", model.BackupTypeFull, metrics.OutcomeFailure)
	reporter.Report(
		"routine-fail",
		model.BackupTypeFull,
		time.Minute,
		errors.New("backup failed"),
		logger,
	)
	require.Equal(t, before+1, backupEventCount(t, "routine-fail", model.BackupTypeFull, metrics.OutcomeFailure))
	require.Contains(t, logBuf.String(), "full backup failed")
}

func TestBackupReporter_Report_AerospikeFailure(t *testing.T) {
	reporter := NewBackupReporter()
	logger, logBuf := newTestLogger(t)

	aerr := &as.AerospikeError{ResultCode: astypes.KEY_NOT_FOUND_ERROR}
	before := backupEventCount(t, "routine-as-fail", model.BackupTypeIncremental, metrics.OutcomeFailure)
	reporter.Report(
		"routine-as-fail",
		model.BackupTypeIncremental,
		time.Minute,
		aerr,
		logger,
	)
	require.Equal(t, before+1, backupEventCount(t, "routine-as-fail", model.BackupTypeIncremental, metrics.OutcomeFailure))
	require.Contains(t, logBuf.String(), "incremental backup failed due to Aerospike error")
}

// backupEventCount returns the value of the backup events counter for the given labels,
// or 0 if that series has not been created yet. The default registry persists across
// -count runs, so tests compare the counter value rather than the number of series.
func backupEventCount(t *testing.T, routine string, backupType model.BackupType, outcome metrics.Outcome) int {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	want := map[string]string{"routine": routine, "type": string(backupType), "outcome": string(outcome)}
	for _, family := range families {
		if family.GetName() != "aerospike_backup_service_backup_events_total" {
			continue
		}
		for _, metric := range family.GetMetric() {
			got := make(map[string]string, len(metric.GetLabel()))
			for _, label := range metric.GetLabel() {
				got[label.GetName()] = label.GetValue()
			}
			if maps.Equal(got, want) {
				return int(metric.GetCounter().GetValue())
			}
		}
	}
	return 0
}
