package configuration

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/aerospike/aerospike-backup-service/pkg/model"
)

// HTTPConfigurationManager implements the ConfigurationManager interface,
// performing I/O operations via the HTTP(S) protocol.
type HTTPConfigurationManager struct {
	configURL string
}

var _ ConfigurationManager = (*HTTPConfigurationManager)(nil)

// NewHTTPConfigurationManager returns a new HTTPConfigurationManager.
func NewHTTPConfigurationManager(uri string) ConfigurationManager {
	return &HTTPConfigurationManager{configURL: uri}
}

// ReadConfiguration returns a reader for the configuration using a URL.
func (h *HTTPConfigurationManager) ReadConfiguration() (io.ReadCloser, error) {
	resp, err := http.Get(h.configURL)
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

// WriteConfiguration is unsupported for HTTPConfigurationManager.
func (h *HTTPConfigurationManager) WriteConfiguration(_ *model.Config) error {
	return fmt.Errorf("%w: HTTPConfigurationManager.WriteConfiguration",
		errors.ErrUnsupported)
}
