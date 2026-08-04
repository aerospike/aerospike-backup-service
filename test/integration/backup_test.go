//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFullBackupEmptyNamespaceToLocalStorage(t *testing.T) {
	e := setupEnv(t)

	triggerFullBackup(t, e.server.URL, routineName)

	backups := waitForFullBackupCount(t, e.server.URL, routineName, 1, 60*time.Second)

	assert.Equal(t, namespace, backups[0].Namespace)
	assert.Equal(t, uint64(0), backups[0].RecordCount)
	assert.False(t, backups[0].Created.IsZero())
	assert.False(t, backups[0].Finished.IsZero())
	assert.NotEmpty(t, backups[0].Key)

	metadataPath := filepath.Join(e.backupDir, backups[0].Key, "metadata.yaml")
	_, err := os.Stat(metadataPath)
	require.NoError(t, err, "metadata.yaml should exist at %s", metadataPath)
}
