//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
)

const (
	// backupTimeout bounds how long a test waits for an ad-hoc backup to appear.
	backupTimeout = 6 * time.Second
	// pollInterval is how often the service is asked for the current backup list.
	pollInterval = 250 * time.Millisecond
)

// fullBackupsURL is the ad-hoc full backup endpoint of the routine created by baseConfig.
func (e *env) fullBackupsURL() string {
	return fmt.Sprintf("%s/v1/backups/full/%s", e.server.URL, routineName)
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

// waitForFullBackup polls the routine until it reports exactly one full backup, and returns it.
func (s *Suite) waitForFullBackup(e *env) dto.BackupDetails {
	deadline := time.Now().Add(backupTimeout)

	for {
		backups := s.getFullBackups(e)
		if len(backups) == 1 {
			return backups[0]
		}

		if time.Now().After(deadline) {
			s.Require().Failf("timed out waiting for full backup",
				"routine %q reported %d full backups after %s, want 1",
				routineName, len(backups), backupTimeout)
		}

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
