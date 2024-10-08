package configuration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"gopkg.in/yaml.v3"
)

type Manager interface {
	ReadConfiguration(ctx context.Context) (io.ReadCloser, error)
	WriteConfiguration(ctx context.Context, config *model.Config) error
}

// Load handles the entire configuration setup process
func Load(ctx context.Context, configFile string, remote bool) (*model.Config, Manager, error) {
	manager, err := newConfigManager(configFile, remote)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create config manager: %w", err)
	}

	reader, err := manager.ReadConfiguration(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read configuration file: %w", err)
	}
	defer reader.Close()

	configBytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read configuration content: %w", err)
	}

	config := dto.NewConfigWithDefaultValues()
	if err := yaml.Unmarshal(configBytes, config); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	modelConfig, err := config.ToModel()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert configuration to model: %w", err)
	}

	return modelConfig, manager, nil
}

func newConfigManager(configFile string, remote bool) (Manager, error) {
	if remote {
		storage, err := readStorage(configFile)
		if err != nil {
			return nil, err
		}

		return NewStorageManager(storage), nil
	}

	isHTTP, err := isHTTPPath(configFile)
	if err != nil {
		return nil, err
	}

	if isHTTP {
		return NewHTTPConfigurationManager(configFile), nil
	}

	return NewFileConfigurationManager(configFile), nil
}

func readStorage(configURI string) (model.Storage, error) {
	content, err := loadFileContent(configURI)
	if err != nil {
		return nil, err
	}

	configStorage := &dto.Storage{}
	err = yaml.Unmarshal(content, configStorage)
	if err != nil {
		return nil, err
	}

	err = configStorage.Validate()
	if err != nil {
		return nil, err
	}
	return configStorage.ToModel(), nil
}

func loadFileContent(configFile string) ([]byte, error) {
	isHTTP, err := isHTTPPath(configFile)
	if err != nil {
		return nil, err
	}
	if isHTTP {
		return readFromHTTP(configFile)
	}
	return os.ReadFile(configFile)
}

func readFromHTTP(url string) ([]byte, error) {
	// #nosec G107
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// isHTTPPath determines whether the specified path is a valid http/https.
func isHTTPPath(path string) (bool, error) {
	uri, err := url.Parse(path)
	if err != nil {
		return false, err
	}

	return uri.Scheme == "http" || uri.Scheme == "https", nil
}
