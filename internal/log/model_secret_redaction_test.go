package log

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
)

// Model credential fields are redact.Secret. Whatever the log format, a logged model struct
// must never carry a literal credential.
func TestHandler_RedactsModelCredentialsInAllFormats(t *testing.T) {
	const (
		s3Key       = "AKIA-literal-key-id"
		s3Secret    = "s3-literal-secret-access-key" //nolint:gosec // fixture
		azureKey    = "azure-literal-account-key"
		adSecret    = "aad-literal-client-secret" //nolint:gosec // fixture
		gcpKey      = `{"type":"service_account","private_key":"gcp-literal-private-key"}`
		clusterPass = "cluster-literal-password"
	)

	backups := []model.BackupDetails{
		{Key: "s3", Storage: &model.S3Storage{Bucket: "b", Auth: &model.S3Authentication{
			KeyIDSecret: s3Key, AccessKeySecret: s3Secret}}},
		{Key: "azure-shared", Storage: &model.AzureStorage{ContainerName: "c",
			Auth: &model.AzureSharedKeyAuth{AccountName: "acc", AccountKey: azureKey}}},
		{Key: "azure-ad", Storage: &model.AzureStorage{ContainerName: "c",
			Auth: &model.AzureADAuth{TenantID: "t", ClientID: "c", ClientSecret: adSecret}}},
		{Key: "gcp", Storage: &model.GcpStorage{BucketName: "b", KeyJSON: gcpKey}},
	}
	creds := &model.Credentials{User: "admin", Password: clusterPass}
	literals := []string{s3Key, s3Secret, azureKey, adSecret, "gcp-literal-private-key", clusterPass}

	for name, mk := range map[string]func(*bytes.Buffer) slog.Handler{
		"json": func(b *bytes.Buffer) slog.Handler {
			return slog.NewJSONHandler(b, &slog.HandlerOptions{ReplaceAttr: handlerReplaceAttr})
		},
		"text": func(b *bytes.Buffer) slog.Handler {
			return slog.NewTextHandler(b, &slog.HandlerOptions{ReplaceAttr: handlerReplaceAttr})
		},
	} {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(mk(&buf))
			for _, b := range backups {
				logger.Info("Restore from backup", slog.Any("backup", b))
			}
			logger.Info("cluster", slog.Any("credentials", creds))
			logger.Info("cluster fmt", slog.String("creds", creds.Password.String()))

			out := buf.String()
			for _, lit := range literals {
				assert.NotContains(t, out, lit, "literal credential must not reach the %s log", name)
			}
			assert.Contains(t, out, "[secret]")
		})
	}
}
