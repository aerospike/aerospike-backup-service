//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

const (
	// backupTimeout bounds how long a test waits for an ad-hoc backup to appear.
	backupTimeout = 5 * time.Second
	// pollInterval is how often the service is asked for the current backup list.
	pollInterval = 250 * time.Millisecond
)

// fullBackupsURL is the ad-hoc full backup endpoint of the routine created by baseConfig.
func (e *env) fullBackupsURL() string {
	return fmt.Sprintf("%s/v1/backups/full/%s", e.server.URL, routineName)
}

// incrementalBackupsURL is the ad-hoc incremental backup endpoint of the routine created by baseConfig.
func (e *env) incrementalBackupsURL() string {
	return fmt.Sprintf("%s/v1/backups/incremental/%s", e.server.URL, routineName)
}

// triggerFullBackup asks the service to run a full backup now.
func (s *Suite) triggerFullBackup(e *env) {
	req, err := http.NewRequestWithContext(s.T().Context(), http.MethodPost, e.fullBackupsURL(), nil)
	s.Require().NoError(err)

	resp, err := http.DefaultClient.Do(req)
	s.Require().NoError(err)

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		s.Require().Failf("failed to trigger full backup", "status %d: %s", resp.StatusCode, body)
	}
}

// triggerIncrementalBackup asks the service to run an incremental backup now.
func (s *Suite) triggerIncrementalBackup(e *env) {
	req, err := http.NewRequestWithContext(s.T().Context(), http.MethodPost, e.incrementalBackupsURL(), nil)
	s.Require().NoError(err)

	resp, err := http.DefaultClient.Do(req)
	s.Require().NoError(err)

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		s.Require().Failf("failed to trigger incremental backup", "status %d: %s", resp.StatusCode, body)
	}
}

// waitForIncrementalBackup polls the routine until it reports the expected number of incremental backups.
func (s *Suite) waitForIncrementalBackup(e *env, want int) dto.BackupDetails {
	deadline := time.Now().Add(backupTimeout)

	for {
		// Fail fast if there are any failure events for this routine
		failCount, _, err := e.backupFailureEventCount(s.T().Context(), model.BackupTypeIncremental)
		if err == nil && failCount > 0 {
			s.Require().Failf("incremental backup failed", "aerospike_backup_service_backup_events_total outcome=failure is %f (> 0)", failCount)
		}

		backups := s.getIncrementalBackups(e)
		if len(backups) == want {
			return backups[want-1]
		}

		if time.Now().After(deadline) {
			s.Require().Failf("timed out waiting for incremental backup",
				"routine %q reported %d incremental backups after %s, want %d",
				routineName, len(backups), backupTimeout, want)
		}

		time.Sleep(pollInterval)
	}
}

// getIncrementalBackups returns the incremental backups the routine currently reports.
func (s *Suite) getIncrementalBackups(e *env) []dto.BackupDetails {
	req, err := http.NewRequestWithContext(s.T().Context(), http.MethodGet, e.incrementalBackupsURL(), nil)
	s.Require().NoError(err)

	resp, err := http.DefaultClient.Do(req)
	s.Require().NoError(err)

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		s.Require().Failf("failed to fetch incremental backups", "status %d: %s", resp.StatusCode, body)
	}

	var backups []dto.BackupDetails
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&backups))

	return backups
}

// waitForFullBackup polls the routine until it reports exactly one full backup, and returns it.
func (s *Suite) waitForFullBackup(e *env) dto.BackupDetails {
	deadline := time.Now().Add(backupTimeout)

	for {
		// Fail fast if there are any failure events for this routine
		failCount, _, err := e.backupFailureEventCount(s.T().Context(), model.BackupTypeFull)
		s.Require().NoError(err, "failed to fetch backup failure events")
		s.Require().Zero(failCount, "full backup failed")

		backups := s.getFullBackups(e)
		if len(backups) == 1 {
			return backups[0]
		}

		s.Require().False(time.Now().After(deadline), "timed out waiting for full backup",
			"routine %q reported %d full backups after %s, want 1",
			routineName, len(backups), backupTimeout)

		time.Sleep(pollInterval)
	}
}

// getFullBackups returns the full backups the routine currently reports.
func (s *Suite) getFullBackups(e *env) []dto.BackupDetails {
	req, err := http.NewRequestWithContext(s.T().Context(), http.MethodGet, e.fullBackupsURL(), nil)
	s.Require().NoError(err)

	resp, err := http.DefaultClient.Do(req)
	s.Require().NoError(err)

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		s.Require().Failf("failed to fetch full backups", "status %d: %s", resp.StatusCode, body)
	}

	var backups []dto.BackupDetails
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&backups))

	return backups
}

// restoreURL returns the ad-hoc restore endpoint.
func (e *env) restoreURL() string {
	return fmt.Sprintf("%s/v1/restore/full", e.server.URL)
}

// restoreTimestampURL returns the restore-by-timestamp endpoint.
func (e *env) restoreTimestampURL() string {
	return fmt.Sprintf("%s/v1/restore/timestamp", e.server.URL)
}

// restoreStatusURL returns the restore status endpoint.
func (e *env) restoreStatusURL(jobID int64) string {
	return fmt.Sprintf("%s/v1/restore/status/%d", e.server.URL, jobID)
}

func (s *Suite) restoreByTimestamp(e *env, timestamp time.Time) dto.RestoreJobStatus {
	restoreReq := dto.RestoreTimestampRequest{
		DestinationClusterConfig: dto.DestinationClusterConfig{
			Name: clusterName,
		},
		Policy:  &dto.TimestampRestorePolicy{},
		Time:    timestamp.UnixMilli(),
		Routine: routineName,
	}

	jobID := s.triggerRestoreByTimestamp(e, restoreReq)

	return s.waitForRestore(e, jobID)
}

func (s *Suite) restoreByPath(e *env, key string) dto.RestoreJobStatus {
	restoreReq := dto.RestoreRequest{
		DestinationClusterConfig: dto.DestinationClusterConfig{
			Name: clusterName,
		},
		StorageConfig: dto.StorageConfig{
			Name: storageName,
		},
		Policy:         &dto.RestorePolicy{},
		BackupDataPath: key,
	}

	jobID := s.triggerRestore(e, restoreReq)

	return s.waitForRestore(e, jobID)
}

// triggerRestoreByTimestamp asks the service to run a restore-by-timestamp job.
func (s *Suite) triggerRestoreByTimestamp(e *env, request dto.RestoreTimestampRequest) int64 {
	requestBody, err := json.Marshal(request)
	s.Require().NoError(err)

	req, err := http.NewRequestWithContext(
		s.T().Context(), http.MethodPost, e.restoreTimestampURL(), bytes.NewReader(requestBody),
	)
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	s.Require().Equal(resp.StatusCode, http.StatusAccepted, "failed to trigger restore by timestamp: %s", body)

	jobID, err := strconv.ParseInt(string(body), 10, 64)
	s.Require().NoError(err)

	return jobID
}

// triggerRestore asks the service to run a restoreByPath now.
func (s *Suite) triggerRestore(e *env, request dto.RestoreRequest) int64 {
	requestBody, err := json.Marshal(request)
	s.Require().NoError(err)

	req, err := http.NewRequestWithContext(s.T().Context(), http.MethodPost, e.restoreURL(), bytes.NewReader(requestBody))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	s.Require().Equal(resp.StatusCode, http.StatusAccepted, "failed to trigger restore")

	jobID, err := strconv.ParseInt(string(body), 10, 64)
	s.Require().NoError(err)

	return jobID
}

// waitForRestore polls the restore status until it completes.
func (s *Suite) waitForRestore(e *env, jobID int64) dto.RestoreJobStatus {
	deadline := time.Now().Add(backupTimeout)

	for {
		req, err := http.NewRequestWithContext(s.T().Context(), http.MethodGet, e.restoreStatusURL(jobID), nil)
		s.Require().NoError(err)

		resp, err := http.DefaultClient.Do(req)
		s.Require().NoError(err)

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			s.Require().Failf("failed to fetch restore status", "status %d: %s", resp.StatusCode, body)
		}

		var status dto.RestoreJobStatus
		s.Require().NoError(json.NewDecoder(resp.Body).Decode(&status))

		if status.Status != dto.RestoreRunning {
			s.Require().Equal(dto.RestoreSuccess, status.Status, "restore job failed with error: %s", status.Error)
			return status
		}

		if time.Now().After(deadline) {
			s.Require().Failf("timed out waiting for restore", "job %d was still running after %s", jobID, backupTimeout)
		}

		time.Sleep(pollInterval)
	}
}
