//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/stretchr/testify/require"
)

func fullBackupsURL(baseURL, routineName string) string {
	return fmt.Sprintf("%s/v1/backups/full/%s", baseURL, routineName)
}

func triggerFullBackup(t *testing.T, baseURL, routineName string) {
	t.Helper()

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		fullBackupsURL(baseURL, routineName),
		nil,
	)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		require.Failf(t, "unexpected status", "status %d: %s", resp.StatusCode, body)
	}
}

func fetchFullBackupsForRoutine(t *testing.T, baseURL, routineName string) []dto.BackupDetails {
	t.Helper()
	return fetchFullBackups(t, fullBackupsURL(baseURL, routineName))
}

func fetchFullBackups(t *testing.T, url string) []dto.BackupDetails {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		require.Failf(t, "unexpected status", "status %d: %s", resp.StatusCode, body)
	}

	var backups []dto.BackupDetails
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&backups))

	return backups
}

func waitForFullBackupCount(
	t *testing.T,
	baseURL, routineName string,
	count int,
	timeout time.Duration,
) []dto.BackupDetails {
	t.Helper()

	var backups []dto.BackupDetails
	require.Eventually(t, func() bool {
		backups = fetchFullBackupsForRoutine(t, baseURL, routineName)
		return len(backups) == count
	}, timeout, 250*time.Millisecond)

	return backups
}
