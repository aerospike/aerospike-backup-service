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

// The "Last existing backup" line must show the time. *model.BackupTime keeps its state in
// unexported fields, which the redaction walk in handlerReplaceAttr has to carry through.
func TestHandler_LogsBackupTime(t *testing.T) {
	t.Skip("BKRS-416: the redaction walk zeroes unexported fields, so BackupTime renders as {}")

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{ReplaceAttr: handlerReplaceAttr}))

	backupTime := model.NewFullBackupTime(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	logger.Info("Last existing backup", slog.Any("time", backupTime))

	assert.Contains(t, buf.String(), "2026-01-02T03:04:05Z")
	assert.NotContains(t, buf.String(), `"time":{}`)
}

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
