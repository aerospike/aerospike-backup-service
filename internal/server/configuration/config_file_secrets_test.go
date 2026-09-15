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

// Write must not leave the configuration file world-readable: it holds literal cluster and
// cloud credentials, and an older install may have created it under umask 002 (0644).
func TestLocalManager_WriteTightensFileMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "abs.yml")
	require.NoError(t, os.WriteFile(path, []byte("{}\n"), 0o644)) //nolint:gosec // the pre-existing mode under test

	require.NoError(t, localManager(path).Write(t.Context(), configWithSecret(t)))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(data), "literal-cluster-password", "the file holds literal secrets")

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(configFileMode), info.Mode().Perm())
}

// A new configuration file is created with the same restricted mode.
func TestLocalManager_WriteCreatesFileWithTightMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "abs.yml")

	require.NoError(t, localManager(path).Write(t.Context(), configWithSecret(t)))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(configFileMode), info.Mode().Perm())
}

// A failed write leaves the previous configuration readable instead of an empty or torn file,
// and drops the temporary file it was building.
func TestLocalManager_WriteFailureKeepsPreviousConfiguration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "abs.yml")
	previous := "# previous configuration\n"
	require.NoError(t, os.WriteFile(path, []byte(previous), configFileMode))

	// A read-only directory admits no temporary file, so the write fails before publishing.
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	err := localManager(path).Write(t.Context(), configWithSecret(t))
	require.Error(t, err)

	data, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, previous, string(data), "the previous configuration was damaged")

	entries, readErr := os.ReadDir(dir)
	require.NoError(t, readErr)
	assert.Len(t, entries, 1, "a temporary file was left behind")
}

// The target is replaced rather than truncated in place, so a reader holding the old file
// keeps seeing a complete configuration.
func TestLocalManager_WriteReplacesTheFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "abs.yml")
	require.NoError(t, os.WriteFile(path, []byte("# previous configuration\n"), configFileMode))

	before, err := os.Stat(path)
	require.NoError(t, err)

	require.NoError(t, localManager(path).Write(t.Context(), configWithSecret(t)))

	after, err := os.Stat(path)
	require.NoError(t, err)
	assert.False(t, os.SameFile(before, after), "the file was written through, not replaced")

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "a temporary file was left behind")
}

// The in-place fallback, taken when the target cannot be replaced by a rename (a
// bind-mounted configuration file, for instance), writes the configuration and tightens the
// mode it finds.
func TestLocalManager_WriteInPlaceTightensFileMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "abs.yml")
	require.NoError(t, os.WriteFile(path, []byte("{}\n"), 0o644)) //nolint:gosec // the pre-existing mode under test

	require.NoError(t, localManager(path).writeInPlace(configWithSecret(t)))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(data), "literal-cluster-password")

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(configFileMode), info.Mode().Perm())
}

func localManager(path string) *fileConfigurationManager {
	var nsValidator aerospike.NamespaceValidator // Write never consults it

	return &fileConfigurationManager{FilePath: path, nsValidator: nsValidator}
}

func configWithSecret(t *testing.T) *model.Config {
	t.Helper()

	config := model.NewConfig()
	require.NoError(t, config.AddCluster("c", &model.AerospikeCluster{
		SeedNodes:   []model.SeedNode{{HostName: "localhost", Port: 3000}},
		Credentials: &model.Credentials{User: "admin", Password: "literal-cluster-password"},
	}))

	return config
}
