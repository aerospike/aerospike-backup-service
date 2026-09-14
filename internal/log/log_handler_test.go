package log

import (
	"bytes"
	"log/slog"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
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
	assert.Contains(t, output, `"password":"[secret]"`)
	assert.NotContains(t, output, "superSecretPassword")
}

func TestHandlerReplaceAttr_RedactsModelStorageCredentials(t *testing.T) {
	const (
		s3Literal    = "AWS-SECRET-ACCESS-KEY-VALUE"
		azureLiteral = "AZURE-ACCOUNT-KEY-VALUE"
		adLiteral    = "AZURE-CLIENT-SECRET-VALUE"
		gcpKeyJSON   = `{"type":"service_account","private_key":"-----BEGIN PRIVATE KEY-----"}`
	)

	storages := map[string]struct {
		storage model.Storage
		secret  string
	}{
		"s3": {
			storage: &model.S3Storage{Bucket: "b", Auth: &model.S3Authentication{
				KeyIDSecret: "AKIA", AccessKeySecret: s3Literal,
			}},
			secret: s3Literal,
		},
		"azure shared key": {
			storage: &model.AzureStorage{ContainerName: "c", Auth: &model.AzureSharedKeyAuth{
				AccountName: "acc", AccountKey: azureLiteral,
			}},
			secret: azureLiteral,
		},
		"azure ad": {
			storage: &model.AzureStorage{ContainerName: "c", Auth: &model.AzureADAuth{
				TenantID: "tenant", ClientID: "client", ClientSecret: adLiteral,
			}},
			secret: adLiteral,
		},
		"gcp": {
			storage: &model.GcpStorage{BucketName: "b", KeyJSON: gcpKeyJSON},
			secret:  "BEGIN PRIVATE KEY",
		},
	}

	handlers := map[string]func(w *bytes.Buffer) slog.Handler{
		"json": func(w *bytes.Buffer) slog.Handler {
			return slog.NewJSONHandler(w, &slog.HandlerOptions{ReplaceAttr: handlerReplaceAttr})
		},
		"plain": func(w *bytes.Buffer) slog.Handler {
			return slog.NewTextHandler(w, &slog.HandlerOptions{ReplaceAttr: handlerReplaceAttr})
		},
	}

	for format, newHandler := range handlers {
		for name, test := range storages {
			t.Run(format+"/"+name, func(t *testing.T) {
				var buf bytes.Buffer
				log := slog.New(newHandler(&buf))

				details := model.NewBackupDetails(
					model.BackupMetadata{Created: time.Now(), Namespace: "ns"},
					"routine/backup/123/data/ns",
					test.storage,
				)

				log.Info("Start restoring", slog.Any("backup", details))
				log.Info("Start restoring", slog.Any("backups", []model.BackupDetails{details}))

				assert.Contains(t, buf.String(), `routine/backup/123/data/ns`)
				assert.NotContains(t, buf.String(), test.secret)
			})
		}
	}
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
	assert.NotContains(t, output, `"password":"[secret]"`)
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

func TestHandlerReplaceAttr_BackupTime(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		ReplaceAttr: handlerReplaceAttr,
	}))

	backupTime := model.NewFullBackupTime(time.Now())
	log.Info("Last existing backup", slog.Any("time", backupTime))

	assert.Contains(t, buf.String(), `"msg":"Last existing backup"`)
}
