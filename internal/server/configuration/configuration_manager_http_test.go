package configuration

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestHTTPConfigurationManager_Read(t *testing.T) {
	validYAML := "service:\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/valid.yaml":
			_, _ = fmt.Fprint(w, validYAML)
		case "/invalid.yaml":
			_, _ = fmt.Fprint(w, "invalid: [yaml")
		case "/not-found.yaml":
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	tests := []struct {
		name        string
		configURL   string
		expectError string
	}{
		{
			name:        "missing url",
			configURL:   "",
			expectError: "configuration URL is missing",
		},
		{
			name:        "request creation failure",
			configURL:   "://bad-url",
			expectError: "failed to create HTTP request",
		},
		{
			name:        "http do failure",
			configURL:   "http://127.0.0.1:0/unreachable",
			expectError: "failed to execute HTTP request",
		},
		{
			name:        "non-200 status",
			configURL:   server.URL + "/not-found.yaml",
			expectError: "unexpected HTTP status code",
		},
		{
			name:        "invalid yaml body",
			configURL:   server.URL + "/invalid.yaml",
			expectError: "failed to unmarshal configuration",
		},
		{
			name:      "success",
			configURL: server.URL + "/valid.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockNsValidator := aerospike.NewMockNamespaceValidator(ctrl)
			mockNsValidator.EXPECT().Validate(gomock.Any(), gomock.Any()).AnyTimes()

			manager := newHTTPConfigurationManager(tt.configURL, mockNsValidator)
			cfg, err := manager.Read(t.Context())

			if tt.expectError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectError)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, cfg)
		})
	}
}

func TestHTTPConfigurationManager_Read_ContextCanceled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockNsValidator := aerospike.NewMockNamespaceValidator(ctrl)

	manager := newHTTPConfigurationManager("http://example.com/config.yaml", mockNsValidator)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := manager.Read(ctx)
	require.Error(t, err)
}

func TestHTTPConfigurationManager_Write(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockNsValidator := aerospike.NewMockNamespaceValidator(ctrl)

	manager := newHTTPConfigurationManager("http://example.com/config.yaml", mockNsValidator)

	err := manager.Write(t.Context(), model.NewConfig())
	require.Error(t, err)
	require.Contains(t, err.Error(), "writing configuration is not supported for HTTP")
}
