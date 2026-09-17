package log

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NewHandler probes the log file when it is built: lumberjack opens the file lazily and slog
// drops write errors, so an unwritable destination has to fail here rather than leave the
// service running without any log output.
func TestNewHandler_RejectsUnwritableLogFile(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(blocker, nil, 0o600))

	_, err := NewHandler(&model.LoggerConfig{
		Level:      model.LogLevelInfo,
		Format:     model.LogFormatPlain,
		FileWriter: &model.FileLoggerConfig{Filename: filepath.Join(blocker, "abs.log")},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not writable")

	handler, err := NewHandler(&model.LoggerConfig{
		Level:      model.LogLevelInfo,
		Format:     model.LogFormatPlain,
		FileWriter: &model.FileLoggerConfig{Filename: filepath.Join(t.TempDir(), "logs", "abs.log")},
	})
	require.NoError(t, err)
	assert.NotNil(t, handler)
}

// The "Last existing backup" line must show the time. *model.BackupTime keeps its state in
// unexported fields, which the redaction walk in handlerReplaceAttr has to carry through.
// The service logs in plain format by default, where slog renders the value through its
// Stringer.
func TestHandler_LogsBackupTime(t *testing.T) {
	backupTime := model.NewFullBackupTime(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))

	var redacted bytes.Buffer
	logTo(t, slog.NewTextHandler(&redacted, &slog.HandlerOptions{ReplaceAttr: handlerReplaceAttr}), backupTime)

	assert.Contains(t, redacted.String(), "2026-01-02T03:04:05Z")
	assert.NotContains(t, redacted.String(), "Full: never")
}

// More generally: a value that carries no secret reaches the log exactly as it would
// without the redaction walk.
func TestHandler_RedactionLeavesSecretFreeValuesUntouched(t *testing.T) {
	backupTime := model.NewFullBackupTime(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))

	var redacted, plain bytes.Buffer
	logTo(t, slog.NewTextHandler(&redacted, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			return handlerReplaceAttr(groups, dropRecordTime(groups, a))
		},
	}), backupTime)
	logTo(t, slog.NewTextHandler(&plain, &slog.HandlerOptions{ReplaceAttr: dropRecordTime}), backupTime)

	assert.Equal(t, plain.String(), redacted.String())
}

func logTo(t *testing.T, handler slog.Handler, backupTime *model.BackupTime) {
	t.Helper()

	slog.New(handler).Info("Last existing backup", slog.Any("time", backupTime))
}

// dropRecordTime removes the wall-clock attribute slog adds to every record, so two records
// logged at different moments can be compared. The attribute the test logs shares its key, so
// the value kind is what tells them apart.
func dropRecordTime(groups []string, a slog.Attr) slog.Attr {
	if len(groups) == 0 && a.Key == slog.TimeKey && a.Value.Kind() == slog.KindTime {
		return slog.Attr{}
	}

	return a
}
