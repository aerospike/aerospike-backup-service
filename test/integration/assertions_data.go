//go:build integration

// Data assertions for integration tests: backup API responses.
//
// Keep assertData* helpers here rather than in *_test.go files so scenarios stay focused on
// setup and flow while data checks remain reusable across future tests.
package integration

import (
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	as "github.com/aerospike/aerospike-client-go/v8"
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

// assertRecordsRestored verifies that a record with the given age exists (in setName) for each
// entry, keyed by its position in ages, matching the layout written by seedRecords.
func (s *Suite) assertRecordsRestored(ages []int) {
	s.T().Helper()

	for i, age := range ages {
		key, err := as.NewKey(namespace, setName, i)
		s.Require().NoError(err)

		record, err := s.client.Get(nil, key)
		s.Require().NoError(err)
		s.Require().NotNil(record)
		s.Equal(age, record.Bins["age"])
	}
}
