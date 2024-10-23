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
	storageDto := "local-storage:\n    path: ./config.yaml"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/storage.yaml":
			_, _ = fmt.Fprint(w, storageDto)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	tests := []struct {
		name         string
		configFile   string
		remote       bool
		setup        func() error
		expectError  bool
		expectedType reflect.Type
	}{
		{
			name:       "Local file, non-remote",
			configFile: filepath.Join(tempDir, "local_config.yaml"),
			remote:     false,
			setup: func() error {
				return os.WriteFile(filepath.Join(tempDir, "local_config.yaml"), []byte("test: config"), 0600)
			},
			expectError:  false,
			expectedType: reflect.TypeOf(&fileConfigurationManager{}),
		},
		{
			name:         "HTTP file, non-remote",
			configFile:   server.URL + "/config.yaml",
			remote:       false,
			setup:        func() error { return nil },
			expectError:  false,
			expectedType: reflect.TypeOf(&httpConfigurationManager{}),
		},
		{
			name:         "HTTP file, remote",
			configFile:   server.URL + "/storage.yaml",
			remote:       true,
			setup:        func() error { return nil },
			expectError:  false,
			expectedType: reflect.TypeOf(&storageManager{}),
		},
		{
			name:       "Local file, remote",
			configFile: filepath.Join(tempDir, "storage_config.yaml"),
			remote:     true,
			setup: func() error {
				return os.WriteFile(filepath.Join(tempDir, "storage_config.yaml"), []byte(storageDto), 0600)
			},
			expectError:  false,
			expectedType: reflect.TypeOf(&storageManager{}),
		},
		{
			name:       "Local file, remote but invalid storage config",
			configFile: filepath.Join(tempDir, "invalid_storage.yaml"),
			remote:     true,
			setup: func() error {
				return os.WriteFile(filepath.Join(tempDir, "invalid_storage.yaml"), []byte("invalid: yaml"), 0600)
			},
			expectError:  true,
			expectedType: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.setup()
			require.NoError(t, err, "Setup failed")

			config, err := newConfigManager(tt.configFile, tt.remote, nil)
			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			configType := reflect.TypeOf(config)
			require.Equal(t, tt.expectedType.String(), configType.String())
		})
	}
}
