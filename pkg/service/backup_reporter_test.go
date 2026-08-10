package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	as "github.com/aerospike/aerospike-client-go/v8"
	astypes "github.com/aerospike/aerospike-client-go/v8/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

func TestBackupReporter_Report_Success(t *testing.T) {
	reporter := NewBackupReporter()
	start := time.Now()
	duration := 2 * time.Minute

	before := backupEventCount(t)
	reporter.Report(
		"routine-success",
		model.BackupTypeFull,
		start,
		duration,
		nil,
		slog.New(slog.DiscardHandler),
	)
	require.Equal(t, before+1, backupEventCount(t))
}

func TestBackupReporter_Report_Skipped(t *testing.T) {
	reporter := NewBackupReporter()

	before := backupEventCount(t)
	reporter.Report(
		"routine-skip",
		model.BackupTypeIncremental,
		time.Now(),
		time.Minute,
		fmt.Errorf("wrapped: %w", errBackupSkipped),
		slog.New(slog.DiscardHandler),
	)
	require.Equal(t, before+1, backupEventCount(t))
}

func TestBackupReporter_Report_Canceled(t *testing.T) {
	reporter := NewBackupReporter()

	before := backupEventCount(t)
	reporter.Report(
		"routine-cancel",
		model.BackupTypeFull,
		time.Now(),
		30*time.Second,
		context.Canceled,
		slog.New(slog.DiscardHandler),
	)
	require.Equal(t, before+1, backupEventCount(t))
}

func TestBackupReporter_Report_DeadlineExceeded(t *testing.T) {
	reporter := NewBackupReporter()

	before := backupEventCount(t)
	reporter.Report(
		"routine-deadline",
		model.BackupTypeFull,
		time.Now(),
		30*time.Second,
		context.DeadlineExceeded,
		slog.New(slog.DiscardHandler),
	)
	require.Equal(t, before+1, backupEventCount(t))
}

func TestBackupReporter_Report_GenericFailure(t *testing.T) {
	reporter := NewBackupReporter()
	capture := &slogCaptureHandler{}
	logger := slog.New(capture)

	before := backupEventCount(t)
	reporter.Report(
		"routine-fail",
		model.BackupTypeFull,
		time.Now(),
		time.Minute,
		errors.New("backup failed"),
		logger,
	)
	require.Equal(t, before+1, backupEventCount(t))
	require.Eventually(t, func() bool {
		return capture.containsMessage("full backup failed")
	}, time.Second, 10*time.Millisecond)
}

func TestBackupReporter_Report_AerospikeFailure(t *testing.T) {
	reporter := NewBackupReporter()
	capture := &slogCaptureHandler{}
	logger := slog.New(capture)

	aerr := &as.AerospikeError{ResultCode: astypes.KEY_NOT_FOUND_ERROR}
	before := backupEventCount(t)
	reporter.Report(
		"routine-as-fail",
		model.BackupTypeIncremental,
		time.Now(),
		time.Minute,
		aerr,
		logger,
	)
	require.Equal(t, before+1, backupEventCount(t))
	require.Eventually(t, func() bool {
		return capture.containsMessage("incremental backup failed due to Aerospike error")
	}, time.Second, 10*time.Millisecond)
}

func backupEventCount(t *testing.T) int {
	t.Helper()
	count, err := testutil.GatherAndCount(prometheus.DefaultGatherer, "aerospike_backup_service_backup_events_total")
	require.NoError(t, err)
	return count
}
