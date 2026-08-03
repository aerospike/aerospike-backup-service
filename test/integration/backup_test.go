//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFullBackupEmptyNamespaceToLocalStorage(t *testing.T) {
	e := setupEnv(t)

	require.NoError(t, triggerFullBackup(t, e.server.URL, routineName))

	var backups []dto.BackupDetails
	var err error

	require.Eventually(t, func() bool {
		backups, err = fetchFullBackupsForRoutine(t, e.server.URL, routineName)
		return err == nil && len(backups) == 1
	}, 60*time.Second, 250*time.Millisecond)

	require.NoError(t, err)
	assert.Equal(t, namespace, backups[0].Namespace)
	assert.Equal(t, uint64(0), backups[0].RecordCount)
	assert.False(t, backups[0].Created.IsZero())
	assert.False(t, backups[0].Finished.IsZero())
	assert.NotEmpty(t, backups[0].Key)

	metadataPath := filepath.Join(e.backupDir, backups[0].Key, "metadata.yaml")
	_, err = os.Stat(metadataPath)
	require.NoError(t, err, "metadata.yaml should exist at %s", metadataPath)
}
