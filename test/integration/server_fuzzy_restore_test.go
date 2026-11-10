//go:build integration

package integration

import (
	"net/http"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// TestServerFuzzyRestoreIntoSource backs up the source cluster, then updates one record, deletes
// another and adds a third, and restores the backup in fuzzy mode into the source cluster
// itself, which stays live. The updated and deleted records get their backed-up values back;
// the added record, which the backup does not hold, stays. The restore sets a retry policy, which
// the server accepts for a fuzzy restore, and its status counts the records of the backup.
func (s *ServerBackupSuite) TestServerFuzzyRestoreIntoSource() {
	e := s.setupEnv(s.serverBackupConfig)
	s.seedRecordsIn(s.client, serverNamespace, []int{10, 20, 30})

	s.triggerFullBackup(e)
	fullBackup := s.waitForFinishedBackup(e, model.BackupTypeFull)

	s.putRecordIn(s.client, serverNamespace, 0, 99)
	s.deleteRecordIn(s.client, serverNamespace, 1)
	s.putRecordIn(s.client, serverNamespace, 3, 40)

	status := s.restoreByPathInto(e, fullBackup.Key, clusterName,
		&dto.RestorePolicy{BaseRestorePolicy: dto.BaseRestorePolicy{
			Mode:        dto.RestoreModeFuzzy,
			RetryPolicy: &dto.RetryPolicy{BaseTimeout: new(int64(200)), Multiplier: new(1.5), MaxRetries: new(2)},
		}})

	s.Equal(dto.RestoreSuccess, status.Status)
	// The server counts a record once per lane it reads it from (SERVER-1972): three or more.
	s.GreaterOrEqual(status.ReadRecords, uint64(3))
	s.assertRecordsIn(s.client, serverNamespace, []int{10, 20, 30, 40})
}

// TestServerFuzzyRestoreByTimestamp backs up a full and an incremental backup of the source
// cluster, updates the record again, and restores the chain by timestamp in fuzzy mode into the
// source cluster: the record ends with its value from the incremental backup.
func (s *ServerBackupSuite) TestServerFuzzyRestoreByTimestamp() {
	const (
		fullAge        = 10
		incrementalAge = 11
	)

	e := s.setupEnv(s.serverBackupConfig)

	s.seedRecordsIn(s.client, serverNamespace, []int{fullAge})
	s.triggerFullBackup(e)
	fullBackup := s.waitForFinishedBackup(e, model.BackupTypeFull)
	s.waitForLastBackup(e, model.BackupTypeFull, fullBackup.Created)

	s.seedRecordsIn(s.client, serverNamespace, []int{incrementalAge})
	s.triggerIncrementalBackup(e)
	s.waitForFinishedBackup(e, model.BackupTypeIncremental)

	s.putRecordIn(s.client, serverNamespace, 0, 99)

	status := s.restoreByTimestampInto(e, time.Now(), clusterName, dto.RestoreModeFuzzy)

	s.Equal(dto.RestoreSuccess, status.Status)
	s.assertRecordsIn(s.client, serverNamespace, []int{incrementalAge})
}

// TestServerRestoreRequestRejected sends restore requests that server mode does not accept: one
// by path and one by timestamp without a mode, which server mode requires, one with a mode that
// does not exist, one with an option the server has no equivalent for, and one that maps the
// backup's namespace onto another. Each is rejected before any job starts.
func (s *ServerBackupSuite) TestServerRestoreRequestRejected() {
	e := s.setupEnv(s.serverBackupConfig)

	byPath := func(policy dto.BaseRestorePolicy) dto.RestoreRequest {
		request := defaultRestoreRequest("261101T120000-aaaa")
		request.DestinationClusterConfig.Name = destinationClusterName
		request.Policy = &dto.RestorePolicy{BaseRestorePolicy: policy}

		return request
	}

	tests := []struct {
		name    string
		path    string
		request any
		wantErr string
	}{
		{
			name:    "by path without a mode",
			path:    "/v1/restore/full",
			request: byPath(dto.BaseRestorePolicy{}),
			wantErr: `"mode" required`,
		},
		{
			name:    "by timestamp without a mode",
			path:    "/v1/restore/timestamp",
			request: timestampRestoreRequest(time.Now(), destinationClusterName, ""),
			wantErr: `"mode" required`,
		},
		{
			name:    "unknown mode",
			path:    "/v1/restore/full",
			request: byPath(dto.BaseRestorePolicy{Mode: "cold"}),
			wantErr: "'cold' is not a valid mode",
		},
		{
			name:    "option the server lacks",
			path:    "/v1/restore/full",
			request: byPath(dto.BaseRestorePolicy{Mode: dto.RestoreModeStrict, Replace: new(true)}),
			wantErr: `restore policy sets replace, which backup-mode "server" does not support`,
		},
		{
			name: "namespace mapping",
			path: "/v1/restore/full",
			request: byPath(dto.BaseRestorePolicy{
				Mode:      dto.RestoreModeStrict,
				Namespace: &dto.RestoreNamespace{Source: serverNamespace, Destination: otherNamespace},
			}),
			wantErr: `restore policy sets a namespace destination other than its source, which backup-mode "server" does not support`,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			body, err := decoder.Marshal(tt.request, decoder.JSON, false)
			s.Require().NoError(err)
			before := s.restoreJobIDs()

			status, response := s.do(e, http.MethodPost, e.baseURL+tt.path, body)

			s.Equal(http.StatusBadRequest, status)
			s.Contains(string(response), tt.wantErr)
			s.Equal(before, s.restoreJobIDs(), "a restore job started")
		})
	}
}
