//go:build integration

package integration

import (
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/aerospike/aerospike-client-go/v8/types"
)

// TestBackupWithFilterExpression backs up with a routine filter-exp of age > 25 and
// checks that only the two matching records out of five are in the backup.
func (s *BackupSuite) TestBackupWithFilterExpression() {
	filterExpression, err := as.ExpGreater(as.ExpIntBin("age"), as.ExpIntVal(25)).Base64()
	s.Require().NoError(err)

	e := s.setupEnv(func(c *dto.Config) {
		r := c.BackupRoutines[routineName]
		r.SetList = []string{setName}
		r.FilterExpression = filterExpression
	})

	s.seedRecords([]int{10, 20, 30, 40, 25})

	s.triggerFullBackup(e)

	s.assertBackupDetails(s.waitForFullBackup(e), 2)
}

// TestClusterRejectsUnparsableFilterAsParameterError pins the server behavior ABS's
// retry classification rests on. Configuration only checks that filter-exp is
// base64, so an expression the server cannot parse does reach the cluster, and it
// must come back as PARAMETER_ERROR: the code pkg/util/try treats as not retryable
// (unit-tested in retry_test.go). It scans with the Aerospike client directly;
// ABS is not involved.
func (s *BackupSuite) TestClusterRejectsUnparsableFilterAsParameterError() {
	s.seedRecords([]int{10, 20, 30})

	garbage, asErr := as.ExpFromBase64("/////////////////////w==")
	s.Require().Nil(asErr)

	policy := as.NewScanPolicy()
	policy.FilterExpression = garbage

	recordset, asErr := s.client.ScanAll(policy, namespace, setName)
	s.Require().Nil(asErr)

	var scanErr error
	for result := range recordset.Results() {
		if result.Err != nil {
			scanErr = result.Err
			break
		}
	}
	s.Require().Error(scanErr, "an unparsable filter expression should fail the scan")

	var asError *as.AerospikeError
	s.Require().ErrorAs(scanErr, &asError)
	s.Truef(asError.Matches(types.PARAMETER_ERROR),
		"expected PARAMETER_ERROR, got %s", types.ResultCodeToString(asError.ResultCode))
}
