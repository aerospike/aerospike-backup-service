package configuration

import (
	"context"
	"errors"
	"fmt"
	"io"
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
func (h *HTTPConfigurationManager) ReadConfiguration(ctx context.Context) (io.ReadCloser, error) {
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

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected HTTP status code: %d", resp.StatusCode)
	}

	return resp.Body, nil
}

// WriteConfiguration is unsupported for HTTPConfigurationManager.
func (h *HTTPConfigurationManager) WriteConfiguration(_ context.Context, _ *model.Config) error {
	return fmt.Errorf("writing configuration is not supported for HTTP: %w", errors.ErrUnsupported)
}
