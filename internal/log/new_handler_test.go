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
