//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
)

func fullBackupsURL(baseURL, routineName string) string {
	return fmt.Sprintf("%s/v1/backups/full/%s", baseURL, routineName)
}

func triggerFullBackup(t *testing.T, baseURL, routineName string) error {
	t.Helper()

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		fullBackupsURL(baseURL, routineName),
		nil,
	)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, body)
	}

	return nil
}

func fetchFullBackupsForRoutine(t *testing.T, baseURL, routineName string) ([]dto.BackupDetails, error) {
	t.Helper()
	return fetchFullBackups(t, fullBackupsURL(baseURL, routineName))
}

func fetchFullBackups(t *testing.T, url string) ([]dto.BackupDetails, error) {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, body)
	}

	var backups []dto.BackupDetails
	if err := json.NewDecoder(resp.Body).Decode(&backups); err != nil {
		return nil, err
	}

	return backups, nil
}
