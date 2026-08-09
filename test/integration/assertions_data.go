//go:build integration

// Data assertions for integration tests: backup API responses.
//
// Keep assertData* helpers here rather than in *_test.go files so scenarios stay focused on
// setup and flow while data checks remain reusable across future tests.
package integration

import (
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
)

func (s *Suite) assertBackupDetails(backup dto.BackupDetails, recordCount uint64) {
	s.T().Helper()

	s.Equal(namespace, backup.Namespace)
	s.Equal(recordCount, backup.RecordCount)
	s.False(backup.Created.IsZero())
	s.False(backup.Finished.IsZero())
	s.NotEmpty(backup.Key)
}

func (s *Suite) assertIncrementalBackupCount(e *env, want int) {
	s.T().Helper()
	s.Len(s.getIncrementalBackups(e), want)
}

// assertBackupListed verifies the backup is returned by GET /v1/backups/full/{name}.
func (s *Suite) assertBackupListed(e *env, want dto.BackupDetails) {
	s.T().Helper()

	for _, backup := range s.getFullBackups(e) {
		if backup.Key != want.Key {
			continue
		}

		s.Equal(want.Created, backup.Created)
		s.assertBackupDetails(backup, want.RecordCount)
		return
	}

	s.Require().FailNow("backup %q not found in full backup list", want.Key)
}

// assertIncrementalBackupListed verifies the backup is returned by GET /v1/backups/incremental/{name}.
func (s *Suite) assertIncrementalBackupListed(e *env, want dto.BackupDetails) {
	s.T().Helper()

	for _, backup := range s.getIncrementalBackups(e) {
		if backup.Key != want.Key {
			continue
		}

		s.Equal(want.Created, backup.Created)
		s.assertBackupDetails(backup, want.RecordCount)
		return
	}

	s.Require().FailNow("backup %q not found in incremental backup list", want.Key)
}
