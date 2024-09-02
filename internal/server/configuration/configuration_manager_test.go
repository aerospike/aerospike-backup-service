package configuration

import (
	"errors"
	"reflect"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockDownloader struct {
	mock.Mock
}

func (m *MockDownloader) read(configFile string) ([]byte, error) {
	args := m.Called(configFile)
	return args.Get(0).([]byte), args.Error(1)
}

type MockS3Builder struct {
}

func (m *MockS3Builder) NewS3ConfigurationManager(storage *model.Storage,
) (ConfigurationManager, error) {
	if storage.Type == model.S3 {
		return &S3ConfigurationManager{}, nil
	}
	return nil, errors.New("wrong type")
}

func TestConfigManagerBuilder_NewConfigManager(t *testing.T) {
	mockLocal := new(MockDownloader)
	mockHTTP := new(MockDownloader)

	tests := []struct {
		name         string
		configFile   string
		remote       bool
		setMock      func()
		expectError  bool
		expectedType reflect.Type
	}{
		// Configuration file is passed straight to the service.
		{
			name:         "local non-remote",
			configFile:   "config.yaml",
			remote:       false,
			setMock:      func() {},
			expectError:  false,
			expectedType: reflect.TypeOf(&FileConfigurationManager{}),
		},
		{
			name:         "http non-remote",
			configFile:   "https://example.com/config.yaml",
			remote:       false,
			setMock:      func() {},
			expectError:  false,
			expectedType: reflect.TypeOf(&HTTPConfigurationManager{}),
		},
		// Open/download remote config file, and based on it's content
		// open/download backup config.
		{
			name:       "local remote local configuration",
			configFile: "/path/to/remote.yaml",
			remote:     true,
			setMock: func() {
				mockLocal.
					On("read", "/path/to/remote.yaml").
					Return([]byte("type: local\npath: config.yaml"), nil)
			},
			expectError:  false,
			expectedType: reflect.TypeOf(&FileConfigurationManager{}),
		},
		{
			name:       "local remote http configuration",
			configFile: "/path/to/remote.yaml",
			remote:     true,
			setMock: func() {
				mockLocal.
					On("read", "/path/to/remote.yaml").
					Return([]byte("type: local\npath: https://example.com/config.yaml"), nil)
			},
			expectError:  false,
			expectedType: reflect.TypeOf(&HTTPConfigurationManager{}),
		},
		{
			name:       "http remote local configuration",
			configFile: "https://example.com/config.yaml",
			remote:     true,
			setMock: func() {
				mockHTTP.
					On("read", "https://example.com/config.yaml").
					Return([]byte("type: local\npath: config.yaml"), nil)
			},
			expectError:  false,
			expectedType: reflect.TypeOf(&FileConfigurationManager{}),
		},
		{
			name:       "http remote http",
			configFile: "http://path/to/remote.yaml",
			remote:     true,
			setMock: func() {
				mockHTTP.
					On("read", "http://path/to/remote.yaml").
					Return([]byte("type: local\npath: https://example.com/config.yaml"), nil)
			},
			expectError:  false,
			expectedType: reflect.TypeOf(&HTTPConfigurationManager{}),
		},
		// S3 configuration, we can read it from local file or by http.
		{
			name:       "local s3",
			configFile: "config.yaml",
			remote:     true,
			setMock: func() {
				mockLocal.
					On("read", "config.yaml").
					Return([]byte("type: aws-s3\npath: s3://bucket/config.yaml\ns3-region: europe"), nil)
			},
			expectError:  false,
			expectedType: reflect.TypeOf(&S3ConfigurationManager{}),
		},
		{
			name:       "http s3",
			configFile: "https://example.com/config.yaml",
			remote:     true,
			setMock: func() {
				mockHTTP.
					On("read", "https://example.com/config.yaml").
					Return([]byte("type: aws-s3\npath: s3://bucket/config.yaml\ns3-region: europe"), nil)
			},
			expectError:  false,
			expectedType: reflect.TypeOf(&S3ConfigurationManager{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLocal = new(MockDownloader)
			mockHTTP = new(MockDownloader)
			tt.setMock()
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
