package configuration

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
)

// httpConfigurationManager implements the Manager interface,
// performing I/O operations via the HTTP(S) protocol.
type httpConfigurationManager struct {
	configURL string
}

var _ Manager = (*httpConfigurationManager)(nil)

// newHTTPConfigurationManager returns a new httpConfigurationManager.
func newHTTPConfigurationManager(uri string) Manager {
	return &httpConfigurationManager{configURL: uri}
}

// ReadConfiguration returns a reader for the configuration using a URL.
func (h *httpConfigurationManager) Read(ctx context.Context) (*model.Config, error) {
	if h.configURL == "" {
		return nil, errors.New("configuration URL is missing")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.configURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute HTTP request: %w", err)
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status code: %d", resp.StatusCode)
	}

	return readConfig(resp.Body)
}

// WriteConfiguration is unsupported for httpConfigurationManager.
func (h *httpConfigurationManager) Write(_ context.Context, _ *model.Config) error {
	return fmt.Errorf("writing configuration is not supported for HTTP: %w", errors.ErrUnsupported)
}
