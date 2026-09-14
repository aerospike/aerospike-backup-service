package configuration

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Write must not leave the configuration file world-readable: it holds literal cluster and
// cloud credentials, and the packaged install creates it under umask 002 (0644).
func TestLocalManager_WriteTightensFileMode(t *testing.T) {
	t.Skip("BKRS-415: Write keeps the mode of a pre-existing world-readable config file")

	path := filepath.Join(t.TempDir(), "abs.yml")
	require.NoError(t, os.WriteFile(path, []byte("{}\n"), 0o644)) //nolint:gosec // the pre-existing mode under test

	cfg := model.NewConfig()
	require.NoError(t, cfg.AddCluster("c", &model.AerospikeCluster{
		SeedNodes:   []model.SeedNode{{HostName: "localhost", Port: 3000}},
		Credentials: &model.Credentials{User: "admin", Password: "literal-cluster-password"},
	}))

	var nsValidator aerospike.NamespaceValidator // Write never consults it
	mgr := newFileConfigurationManager(path, nsValidator)
	require.NoError(t, mgr.Write(t.Context(), cfg))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(data), "literal-cluster-password", "the file holds literal secrets")

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

// Model storage types implement Stringer, so the "%+v" in storageManager.Write's
// error message does not expose cloud credentials.
func TestStorageManager_WriteErrorDoesNotLeakCredentials(t *testing.T) {
	var s model.Storage = &model.S3Storage{
		Bucket: "b",
		Auth:   &model.S3Authentication{KeyIDSecret: "AKIAEXAMPLE", AccessKeySecret: "super-secret-key"},
	}
	msg := fmt.Sprintf("failed to write configuration to storage %+v: %v", s, assert.AnError)
	assert.NotContains(t, msg, "super-secret-key")
	assert.NotContains(t, msg, "AKIAEXAMPLE")

	var az model.Storage = &model.AzureStorage{
		Endpoint: "https://x.blob.core.windows.net", ContainerName: "c",
		Auth: &model.AzureSharedKeyAuth{AccountName: "acct", AccountKey: "azure-account-key"},
	}
	assert.NotContains(t, fmt.Sprintf("%+v", az), "azure-account-key")

	var gcp model.Storage = &model.GcpStorage{BucketName: "b", KeyJSON: `{"private_key":"gcp-private"}`}
	assert.NotContains(t, fmt.Sprintf("%+v", gcp), "gcp-private")
}
