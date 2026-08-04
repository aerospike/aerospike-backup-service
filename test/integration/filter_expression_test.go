//go:build integration

package integration

import (
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	as "github.com/aerospike/aerospike-client-go/v8"
)

func (s *Suite) TestBackupWithFilterExpression() {
	filterExpression, asErr := as.ExpGreater(as.ExpIntBin("age"), as.ExpIntVal(25)).Base64()
	s.Require().NoError(asErr)

	e := s.setupEnv(func(c *dto.Config) {
		r := c.BackupRoutines[routineName]
		r.SetList = []string{setName}
		r.FilterExpression = filterExpression
	})

	s.seedRecords([]int{10, 20, 30, 40, 25})

	s.triggerFullBackup(e)

	backup := s.waitForFullBackup(e)

	s.Equal(namespace, backup.Namespace)
	s.Equal(uint64(2), backup.RecordCount)
}
