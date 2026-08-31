package configuration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	backup "github.com/aerospike/aerospike-backup-service/v3"
	servertls "github.com/aerospike/aerospike-backup-service/v3/internal/server/tlsconfig"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	secrets "github.com/aerospike/aerospike-backup-service/v3/pkg/service/secret"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
)

// Manager reads and writes the whole service configuration in its backing source:
// a local file, an HTTP endpoint, or a storage backend.
type Manager interface {
	// Read reads the configuration from the source.
	Read(ctx context.Context) (*model.Config, error)
	// Write writes the configuration to the source.
	Write(ctx context.Context, config *model.Config) error
}

//nolint:lll
const schemaHeader = "# yaml-language-server: $schema=https://raw.githubusercontent.com/aerospike/aerospike-backup-service/refs/tags/%s/docs/config.schema.json\n"

func Load(
	ctx context.Context,
	configFile string,
	remote bool,
	nsValidator aerospike.NamespaceValidator,
	operations storage.Operations,
	resolver secrets.Resolver,
) (*model.Config, Manager, error) {
	slog.Info("Read service configuration from",
		slog.String("file", configFile),
		slog.Bool("remote", remote))

	manager, err := newConfigManager(ctx, configFile, remote, nsValidator, operations)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create config manager: %w", err)
	}

	config, err := manager.Read(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read configuration: %w", err)
	}
	if err := servertls.ProbeConfig(ctx, config, resolver); err != nil {
		return nil, nil, fmt.Errorf("failed to validate TLS configuration: %w", err)
	}

	return config, manager, nil
}

func readConfig(
	ctx context.Context,
	reader io.Reader,
	nsValidator aerospike.NamespaceValidator,
) (*model.Config, error) {
	configBytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration content: %w", err)
	}

	config := &dto.Config{}
	if err := decoder.Deserialize(config, bytes.NewReader(configBytes), decoder.YAML); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate configuration: %w", err)
	}

	modelConfig, err := config.ToModel()
	if err != nil {
		return nil, fmt.Errorf("failed to convert configuration to model: %w", err)
	}

	nsValidator.Validate(ctx, modelConfig)

	return modelConfig, nil
}

func writeConfig(writer io.Writer, config *model.Config) error {
	dtoConfig := dto.NewConfigFromModel(config)
	data, err := decoder.Marshal(dtoConfig, decoder.YAML, false)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}
	dataWithScheme := append(fmt.Appendf(nil, schemaHeader, backup.Version), data...)
	_, err = writer.Write(dataWithScheme)
	return err
}

func newConfigManager(
	ctx context.Context,
	configFile string,
	remote bool,
	nsValidator aerospike.NamespaceValidator,
	operations storage.Operations,
) (Manager, error) {
	if remote {
		s, err := readStorage(ctx, configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read remote storage configuration: %w", err)
		}
		return newStorageManager(s, nsValidator, operations), nil
	}

	if isHTTPPath(configFile) {
		return newHTTPConfigurationManager(configFile, nsValidator), nil
	}

	return newFileConfigurationManager(configFile, nsValidator), nil
}

func readStorage(ctx context.Context, configURI string) (model.Storage, error) {
	content, err := loadFileContent(ctx, configURI)
	if err != nil {
		return nil, fmt.Errorf("failed to load file content: %w", err)
	}

	configStorage := &dto.Storage{}
	if err = decoder.Deserialize(configStorage, bytes.NewReader(content), decoder.YAML); err != nil {
		return nil, fmt.Errorf("failed to unmarshal storage configuration: %w", err)
	}

	if err = configStorage.Validate(); err != nil {
		return nil, fmt.Errorf("validate storage configuration error: %w", err)
	}

	return configStorage.ToModel(model.NewConfig())
}

func loadFileContent(ctx context.Context, configFile string) ([]byte, error) {
	if isHTTPPath(configFile) {
		return readFromHTTP(ctx, configFile)
	}

	content, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s from disk: %w", configFile, err)
	}

	return content, nil
}

func readFromHTTP(ctx context.Context, url string) ([]byte, error) {
	// #nosec G107
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request for %s: %w", url, err)
	}
	resp, err := http.DefaultClient.Do(req)
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
