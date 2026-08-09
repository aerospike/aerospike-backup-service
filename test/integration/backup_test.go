//go:build integration

package integration

import (
	"os"
	"path/filepath"
)

func (s *Suite) TestFullBackupEmptyNamespaceToLocalStorage() {
	e := s.setupEnv()

	s.triggerFullBackup(e)

	backup := s.waitForFullBackup(e)

	s.Equal(namespace, backup.Namespace)
	s.Equal(uint64(0), backup.RecordCount)
	s.False(backup.Created.IsZero())
	s.False(backup.Finished.IsZero())
	s.NotEmpty(backup.Key)

	metadataPath := filepath.Join(e.backupDir, backup.Key, "metadata.yaml")
	_, err := os.Stat(metadataPath)
	s.Require().NoError(err, "metadata.yaml should exist at %s", metadataPath)

	s.assertLastSuccessfulBackupMetric(e, "full", backup.Created)
}
