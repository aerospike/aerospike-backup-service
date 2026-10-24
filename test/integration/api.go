//go:build integration

package integration

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// backupsURL is the ad-hoc backup endpoint of the routine baseConfig creates: POST
// runs a backup of that type now, GET lists the ones that exist.
func (e *env) backupsURL(backupType model.BackupType) string {
	return fmt.Sprintf("%s/v1/backups/%s/%s", e.baseURL, backupType, routineName)
}

// triggerFullBackup asks ABS to run a full backup now.
func (s *Suite) triggerFullBackup(e *env) {
	s.triggerBackup(e, model.BackupTypeFull)
}

// triggerIncrementalBackup asks ABS to run an incremental backup now.
func (s *Suite) triggerIncrementalBackup(e *env) {
	s.triggerBackup(e, model.BackupTypeIncremental)
}

// waitForFullBackup polls until the full backup triggerFullBackup started is
// listed, and returns it. It fails as soon as a full backup failure is counted in
// the metrics.
func (s *Suite) waitForFullBackup(e *env) dto.BackupDetails {
	return s.waitForNewBackup(e, model.BackupTypeFull)
}

// waitForIncrementalBackup polls until the incremental backup
// triggerIncrementalBackup started is listed, and returns it. It fails as soon as an
// incremental backup failure is counted in the metrics.
func (s *Suite) waitForIncrementalBackup(e *env) dto.BackupDetails {
	return s.waitForNewBackup(e, model.BackupTypeIncremental)
}

// defaultRestoreRequest restores the backup at key from the baseConfig storage into
// the baseConfig cluster.
func defaultRestoreRequest(key string) dto.RestoreRequest {
	return dto.RestoreRequest{
		DestinationClusterConfig: dto.DestinationClusterConfig{
			Name: clusterName,
		},
		StorageConfig: dto.StorageConfig{
			Name: storageName,
		},
		Policy:         &dto.RestorePolicy{},
		BackupDataPath: dto.Path(key),
	}
}

// restoreByPath runs request through POST /v1/restore/full and waits for the job
// to succeed.
func (s *Suite) restoreByPath(e *env, request dto.RestoreRequest) dto.RestoreJobStatus {
	return s.restore(e, e.baseURL+"/v1/restore/full", request)
}

// restoreByTimestamp restores the baseConfig routine's backups up to timestamp into
// the baseConfig cluster through POST /v1/restore/timestamp, and waits for the job
// to succeed.
func (s *Suite) restoreByTimestamp(e *env, timestamp time.Time) dto.RestoreJobStatus {
	return s.restore(e, e.baseURL+"/v1/restore/timestamp", dto.RestoreTimestampRequest{
		DestinationClusterConfig: dto.DestinationClusterConfig{
			Name: clusterName,
		},
		Policy:  &dto.TimestampRestorePolicy{},
		Time:    timestamp.UnixMilli(),
		Routine: routineName,
	})
}

// triggerBackup records when the newest listed backup of backupType was created,
// then starts one.
//
// That time is what waitForNewBackup waits to be passed. Storage may already hold
// backups of the same routine (a bucket shared by several subtests), and retention
// may delete some as new ones land, so neither a fixed count nor "one more than
// before" identifies the backup just started; being created after the newest one
// listed does.
func (s *Suite) triggerBackup(e *env, backupType model.BackupType) {
	e.newestBeforeTrigger[backupType] = s.newestBackupCreated(e, backupType)

	status, body := s.do(e, http.MethodPost, e.backupsURL(backupType), nil)
	s.Require().Equal(http.StatusAccepted, status, "trigger %s backup: %s", backupType, body)
}

func (s *Suite) getBackups(e *env, backupType model.BackupType) []dto.BackupDetails {
	status, body := s.do(e, http.MethodGet, e.backupsURL(backupType), nil)
	s.Require().Equal(http.StatusOK, status, "list %s backups: %s", backupType, body)

	var backups []dto.BackupDetails
	s.Require().NoError(decoder.Deserialize(&backups, bytes.NewReader(body), decoder.JSON))

	return backups
}

// waitForNewBackup polls until a backup of backupType created after the newest one
// listed at the last trigger appears, and returns it.
func (s *Suite) waitForNewBackup(e *env, backupType model.BackupType) dto.BackupDetails {
	after := e.newestBeforeTrigger[backupType]

	var newest dto.BackupDetails

	ok := s.eventually(backupTimeout, pollInterval, func() bool {
		s.Require().Zero(s.metricBackupEventCount(e, backupType, outcomeFailure), "%s backup failed", backupType)

		backups := s.getBackups(e, backupType)
		if len(backups) == 0 {
			return false
		}

		newest = backups[len(backups)-1] // listings are sorted by creation time

		return newest.Created.After(after)
	})
	s.Require().True(ok, "routine %q listed no %s backup created after %s within %s",
		routineName, backupType, after, backupTimeout)

	return newest
}

// newestBackupCreated is when the newest listed backup of backupType was created,
// or the zero time when there is none.
func (s *Suite) newestBackupCreated(e *env, backupType model.BackupType) time.Time {
	backups := s.getBackups(e, backupType)
	if len(backups) == 0 {
		return time.Time{}
	}

	return backups[len(backups)-1].Created
}

// restore posts request to url and waits for the job it starts to succeed.
func (s *Suite) restore(e *env, url string, request any) dto.RestoreJobStatus {
	requestBody, err := decoder.Marshal(request, decoder.JSON, false)
	s.Require().NoError(err)

	status, body := s.do(e, http.MethodPost, url, requestBody)
	s.Require().Equal(http.StatusAccepted, status, "start restore: %s", body)

	jobID, err := strconv.ParseInt(string(body), 10, 64)
	s.Require().NoError(err)

	return s.waitForRestore(e, jobID)
}

// waitForRestore polls the job until it stops running, and fails unless it succeeded.
func (s *Suite) waitForRestore(e *env, jobID int64) dto.RestoreJobStatus {
	url := fmt.Sprintf("%s/v1/restore/status/%d", e.baseURL, jobID)

	var job dto.RestoreJobStatus

	ok := s.eventually(backupTimeout, pollInterval, func() bool {
		status, body := s.do(e, http.MethodGet, url, nil)
		s.Require().Equal(http.StatusOK, status, "restore status: %s", body)
		s.Require().NoError(decoder.Deserialize(&job, bytes.NewReader(body), decoder.JSON))

		return job.Status != dto.RestoreRunning
	})
	s.Require().True(ok, "restore job %d was still running after %s", jobID, backupTimeout)
	s.Require().Equal(dto.RestoreSuccess, job.Status, "restore job failed with error: %s", job.Error)

	return job
}

// do sends one request to ABS and returns the status code and the whole body.
func (s *Suite) do(e *env, method, url string, body []byte) (int, []byte) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(s.T().Context(), method, url, reader)
	s.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := e.client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	return resp.StatusCode, respBody
}
