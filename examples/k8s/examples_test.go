package k8s_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const configKey = "aerospike-backup-service.yml"

type manifest struct {
	Kind string            `yaml:"kind"`
	Data map[string]string `yaml:"data"`
}

// TestExampleConfigsAreValid ensures the ABS config embedded in each example's ConfigMap
// loads and passes the same validation the service applies at startup.
func TestExampleConfigsAreValid(t *testing.T) {
	files, err := filepath.Glob("*.yaml")
	require.NoError(t, err)
	require.NotEmpty(t, files)

	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			content, err := os.ReadFile(file)
			require.NoError(t, err)

			config := embeddedConfig(t, content)

			_, err = dto.NewValidatedFromReader[dto.Config](bytes.NewReader(config), decoder.YAML)
			require.NoError(t, err)
		})
	}
}

func embeddedConfig(t *testing.T, content []byte) []byte {
	t.Helper()

	docs := yaml.NewDecoder(bytes.NewReader(content))
	for {
		var m manifest
		err := docs.Decode(&m)
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)

		if m.Kind == "ConfigMap" {
			config, ok := m.Data[configKey]
			require.True(t, ok, "ConfigMap has no %s", configKey)
			return []byte(config)
		}
	}
	require.FailNow(t, "no ConfigMap found")
	return nil
}
