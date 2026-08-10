//go:build integration

package integration

import (
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
)

// TestRestore runs a full backup, wipes the namespace, restores from that backup, and verifies
// both the restored data and the exposed restore metrics.
func (s *Suite) TestRestore() {
	e := s.setupEnv()

	s.seedRecords([]int{10, 20, 30})

	successCount := s.metricRestoreSuccessEventCount(e)

	s.triggerFullBackup(e)
	fullBackup := s.waitForFullBackup(e)
	s.assertBackupDetails(fullBackup, 3)

	s.Require().NoError(s.client.Truncate(nil, namespace, "", nil))

	status := s.restoreByPath(e, fullBackup.Key)

	s.Equal(dto.RestoreSuccess, status.Status)
	s.Equal(uint64(3), status.InsertedRecords)
	s.Empty(status.Error)

	s.assertRecordsRestored([]int{10, 20, 30})

	s.assertMetricRestoreSuccessEventCount(e, successCount+1)
}
