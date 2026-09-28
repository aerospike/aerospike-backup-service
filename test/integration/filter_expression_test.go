//go:build integration

package integration

import (
	"errors"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/try"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/aerospike/aerospike-client-go/v8/types"
	"github.com/aerospike/backup-go/models"
)

func (s *BackupSuite) TestBackupWithFilterExpression() {
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

// TestUnparsableFilterExpressionIsNotRetried pins the cluster behaviour that the retry
// classification rests on. Configuration only checks that filter-exp is base64, so an
// expression the server cannot parse does reach the cluster; it must come back as
// PARAMETER_ERROR and must not consume the retry budget.
func (s *BackupSuite) TestUnparsableFilterExpressionIsNotRetried() {
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
	s.Require().True(errors.As(scanErr, &asError))
	s.Require().Truef(asError.Matches(types.PARAMETER_ERROR),
		"expected PARAMETER_ERROR, got %s", types.ResultCodeToString(asError.ResultCode))

	attempts := 0
	retryErr := try.Retry(s.T().Context(), models.RetryPolicy{MaxRetries: 3, BaseTimeout: time.Millisecond, Multiplier: 1},
		slog.Default(), func() error {
			attempts++
			return scanErr
		}, func() {})

	s.Require().Error(retryErr)
	s.Equal(1, attempts, "a malformed request must not be retried")
}
