package log

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/reugn/go-quartz/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandlerReplaceAttr_RedactsSecrets(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		ReplaceAttr: handlerReplaceAttr,
	}))

	log.Info("cluster credentials", slog.Any("credentials", &dto.Credentials{
		User:     "testUser",
		Password: "superSecretPassword",
	}))

	output := buf.String()
	assert.Contains(t, output, `"user":"testUser"`)
	assert.Contains(t, output, `"password":`)
	assert.NotContains(t, output, "superSecretPassword")
}

func TestHandlerReplaceAttr_PreservesSecretRef(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		ReplaceAttr: handlerReplaceAttr,
	}))

	log.Info("cluster credentials", slog.Any("credentials", &dto.Credentials{
		User:     "testUser",
		Password: "secrets:resource:key",
	}))

	output := buf.String()
	assert.Contains(t, output, `"password":"secrets:resource:key"`)
}

func TestHandlerReplaceAttr_RendersTraceLevel(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level:       slog.Level(logger.LevelTrace),
		ReplaceAttr: handlerReplaceAttr,
	}))

	log.Log(t.Context(), slog.Level(logger.LevelTrace), "trace message")

	require.Contains(t, buf.String(), `"level":"TRACE"`)
}
