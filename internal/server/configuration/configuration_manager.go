package configuration

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

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
			return nil, fmt.Errorf("failed to read remote storage configuration: %w", err)
		}

		return NewStorageManager(storage), nil
	}

	if isHTTPPath(configFile) {
		return NewHTTPConfigurationManager(configFile), nil
	}

	return NewFileConfigurationManager(configFile), nil
}

func readStorage(configURI string) (model.Storage, error) {
	content, err := loadFileContent(configURI)
	if err != nil {
		return nil, fmt.Errorf("failed to load file content: %w", err)
	}

	configStorage := &dto.Storage{}
	if err = yaml.Unmarshal(content, configStorage); err != nil {
		return nil, fmt.Errorf("failed to unmarshal storage configuration: %w", err)
	}

	if err = configStorage.Validate(); err != nil {
		return nil, fmt.Errorf("validate storage configuration error: %w", err)
	}

	return configStorage.ToModel(), nil
}

func loadFileContent(configFile string) ([]byte, error) {
	if isHTTPPath(configFile) {
		return readFromHTTP(configFile)
	}

	content, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s from disk: %w", configFile, err)
	}

	return content, nil
}

func readFromHTTP(url string) ([]byte, error) {
	// #nosec G107
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed HTTP GET request to %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status code: %d", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read HTTP response body: %w", err)
	}

	return content, nil
}

// isHTTPPath determines whether the specified path is a valid http/https.
func isHTTPPath(path string) bool {
	return strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://")
}
