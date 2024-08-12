package service

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"

	"github.com/aerospike/backup/pkg/model"
	"gopkg.in/yaml.v3"
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

// ReadConfiguration reads and returns the configuration using a URL.
func (h *HTTPConfigurationManager) ReadConfiguration() (*model.Config, error) {
	resp, err := http.Get(h.configURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	config := model.NewConfigWithDefaultValues()
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return nil, err
	}

	respByte := buf.Bytes()
	if err := yaml.Unmarshal(respByte, config); err != nil {
		return nil, err
	}

	return config, nil
}

// WriteConfiguration is unsupported for HTTPConfigurationManager.
func (h *HTTPConfigurationManager) WriteConfiguration(_ *model.Config) error {
	return fmt.Errorf("%w: HTTPConfigurationManager.WriteConfiguration",
		errors.ErrUnsupported)
}
