//go:build integration

// Shared test assertions for integration tests.
//
// Keep assert* helpers here rather than in *_test.go files so scenarios stay focused on
// setup and flow while assertions remain reusable across future backup and metrics tests.
package integration

import (
	"os"
	"path/filepath"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

func (s *Suite) assertBackupDetails(backup dto.BackupDetails, recordCount uint64) {
	s.T().Helper()

	s.Equal(namespace, backup.Namespace)
	s.Equal(recordCount, backup.RecordCount)
	s.False(backup.Created.IsZero())
	s.False(backup.Finished.IsZero())
	s.NotEmpty(backup.Key)
}

func (s *Suite) assertBackupMetadataOnDisk(e *env, backup dto.BackupDetails) {
	s.T().Helper()

	metadataPath := filepath.Join(e.backupDir, backup.Key, "metadata.yaml")
	_, err := os.Stat(metadataPath)
	s.Require().NoError(err, "metadata.yaml should exist at %s", metadataPath)
}

// assertLastSuccessfulBackupMetric polls /metrics until the gauge matches the backup Created time.
func (s *Suite) assertLastSuccessfulBackupMetric(e *env, backupType model.BackupType, want time.Time) {
	s.T().Helper()

	wantUnix := want.Unix()
	deadline := time.Now().Add(backupTimeout)

	for {
		value, ok, err := e.lastSuccessfulBackupTimestamp(s.T().Context(), backupType)
		s.Require().NoError(err)

		if ok && int64(value) == wantUnix {
			return
		}

		if time.Now().After(deadline) {
			if !ok {
				s.Require().Failf("timed out waiting for last successful backup metric",
					"metric %q with routine=%q type=%q not found after %s",
					lastSuccessfulBackupTimestampMetric, routineName, backupType, backupTimeout)
			}

			s.Require().Failf("timed out waiting for last successful backup metric",
				"metric %q with routine=%q type=%q: got %d, want %d after %s",
				lastSuccessfulBackupTimestampMetric, routineName, backupType, int64(value), wantUnix, backupTimeout)
		}

		time.Sleep(pollInterval)
	}
}
