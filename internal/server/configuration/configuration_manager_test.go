package configuration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigManagerBuilder_NewConfigManager(t *testing.T) {
	// Create a temporary directory for local file tests
	tempDir, err := os.MkdirTemp("", "config-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create an HTTP test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/config.yaml":
			fmt.Fprint(w, "type: local\npath: /tmp/config.yaml")
		case "/remote.yaml":
			fmt.Fprint(w, "type: local\npath: https://example.com/config.yaml")
		case "/s3config.yaml":
			fmt.Fprint(w, "type: aws-s3\npath: s3://bucket/config.yaml\ns3-region: europe")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	configPath := filepath.Join(tempDir, "config.yaml")
	tests := []struct {
		name         string
		configFile   string
		remote       bool
		setup        func() error
		expectError  bool
		expectedType reflect.Type
	}{
		{
			name:       "local non-remote",
			configFile: configPath,
			remote:     false,
			setup: func() error {
				return os.WriteFile(configPath, []byte("test config"), 0600)
			},
			expectError:  false,
			expectedType: reflect.TypeOf(&FileConfigurationManager{}),
		},
		{
			name:         "http non-remote",
			configFile:   server.URL + "/config.yaml",
			remote:       false,
			setup:        func() error { return nil },
			expectError:  false,
			expectedType: reflect.TypeOf(&HTTPConfigurationManager{}),
		},
		{
			name:       "local remote local configuration",
			configFile: filepath.Join(tempDir, "remote.yaml"),
			remote:     true,
			setup: func() error {
				return os.WriteFile(filepath.Join(tempDir, "remote.yaml"),
					[]byte(fmt.Sprintf("type: local\npath: %s", configPath)), 0600)
			},
			expectError:  false,
			expectedType: reflect.TypeOf(&FileConfigurationManager{}),
		},
		{
			name:       "local remote http configuration",
			configFile: filepath.Join(tempDir, "remote.yaml"),
			remote:     true,
			setup: func() error {
				return os.WriteFile(filepath.Join(tempDir, "remote.yaml"), []byte(fmt.Sprintf("type: local\npath: %s/config.yaml", server.URL)), 0644)
			},
			expectError:  false,
			expectedType: reflect.TypeOf(&HTTPConfigurationManager{}),
		},
		{
			name:         "http remote local configuration",
			configFile:   server.URL + "/config.yaml",
			remote:       true,
			setup:        func() error { return nil },
			expectError:  false,
			expectedType: reflect.TypeOf(&FileConfigurationManager{}),
		},
		{
			name:         "http remote http",
			configFile:   server.URL + "/remote.yaml",
			remote:       true,
			setup:        func() error { return nil },
			expectError:  false,
			expectedType: reflect.TypeOf(&HTTPConfigurationManager{}),
		},
		{
			name:       "local s3",
			configFile: filepath.Join(tempDir, "s3config.yaml"),
			remote:     true,
			setup: func() error {
				return os.WriteFile(filepath.Join(tempDir, "s3config.yaml"), []byte("type: aws-s3\npath: s3://bucket/config.yaml\ns3-region: europe"), 0644)
			},
			expectError:  false,
			expectedType: reflect.TypeOf(&S3ConfigurationManager{}),
		},
		{
			name:         "http s3",
			configFile:   server.URL + "/s3config.yaml",
			remote:       true,
			setup:        func() error { return nil },
			expectError:  false,
			expectedType: reflect.TypeOf(&S3ConfigurationManager{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.setup()
			require.NoError(t, err, "Setup failed")

			config, err := NewConfigManager(tt.configFile, tt.remote)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			configType := reflect.TypeOf(config)
			require.Equal(t, tt.expectedType.String(), configType.String())
		})
	}
}
