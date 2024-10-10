package configuration

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
)

// HTTPConfigurationManager implements the Manager interface,
// performing I/O operations via the HTTP(S) protocol.
type HTTPConfigurationManager struct {
	configURL string
}

var _ Manager = (*HTTPConfigurationManager)(nil)

// NewHTTPConfigurationManager returns a new HTTPConfigurationManager.
func NewHTTPConfigurationManager(uri string) Manager {
	return &HTTPConfigurationManager{configURL: uri}
}

// ReadConfiguration returns a reader for the configuration using a URL.
func (cm *HTTPConfigurationManager) Read(ctx context.Context) (*model.Config, error) {
	if cm.configURL == "" {
		return nil, errors.New("configuration URL is missing")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cm.configURL, nil)
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

// WriteConfiguration is unsupported for HTTPConfigurationManager.
func (cm *HTTPConfigurationManager) Write(_ context.Context, _ *model.Config) error {
	return fmt.Errorf("writing configuration is not supported for HTTP: %w", errors.ErrUnsupported)
}

func (cm *HTTPConfigurationManager) Update(_ context.Context, _ func(*model.Config) error) error {
	return fmt.Errorf("update is not supported for HTTP: %w", errors.ErrUnsupported)
}
