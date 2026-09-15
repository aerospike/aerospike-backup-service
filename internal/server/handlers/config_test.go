package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/server/configuration"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/redact"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/preflight"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestService_ReadConfig(t *testing.T) {
	svc := &Service{config: model.NewConfig()}
	_ = svc.config.AddCluster("cluster1", &model.AerospikeCluster{})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/config", nil)
	w := httptest.NewRecorder()

	svc.ReadConfig(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response dto.Config
	require.NoError(t, json.NewDecoder(w.Body).Decode(&response))
	assert.Contains(t, response.AerospikeClusters, "cluster1")
}

func newConfigTestService(t *testing.T) (*Service, *gomock.Controller) {
	t.Helper()
	ctrl := gomock.NewController(t)
	checker := preflight.NewMockChecker(ctrl)
	checker.EXPECT().Check(gomock.Any(), gomock.Any()).AnyTimes()

	return &Service{
		config:    model.NewConfig(),
		checker:   checker,
		tlsProber: newMockTLSProber(ctrl),
	}, ctrl
}

func TestService_UpdateConfig(t *testing.T) {
	tests := []struct {
		name             string
		requestBody      string
		configApplierErr error
		writeErr         error
		expectedStatus   int
		expectedError    string
	}{
		{
			name:           "invalid request",
			requestBody:    "{noField : 1}",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid request",
		},
		{
			name:           "static field changed",
			requestBody:    `{"service":{"http":{"port":9999}}}`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "static configuration has changed",
		},
		{
			name:           "write failure",
			requestBody:    `{}`,
			writeErr:       errors.New("write boom"),
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "write boom",
		},
		{
			name:             "apply failure",
			requestBody:      `{}`,
			configApplierErr: errors.New("apply boom"),
			expectedStatus:   http.StatusInternalServerError,
			expectedError:    "apply boom",
		},
		{
			name:           "success",
			requestBody:    `{}`,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, ctrl := newConfigTestService(t)

			mockConfigurationManager := configuration.NewMockManager(ctrl)
			if tt.expectedStatus != http.StatusBadRequest {
				mockConfigurationManager.EXPECT().Write(gomock.Any(), gomock.Any()).Return(tt.writeErr)
			}
			svc.configurationManager = mockConfigurationManager

			mockConfigApplier := service.NewMockConfigApplier(ctrl)
			if tt.expectedStatus == http.StatusOK || tt.configApplierErr != nil {
				mockConfigApplier.EXPECT().ApplyNewConfig().Return(tt.configApplierErr)
			}
			svc.configApplier = mockConfigApplier

			req := httptest.NewRequestWithContext(
				t.Context(), http.MethodPut, "/v1/config", strings.NewReader(tt.requestBody),
			)
			w := httptest.NewRecorder()

			svc.UpdateConfig(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}

func TestService_ApplyConfig(t *testing.T) {
	tests := []struct {
		name             string
		readConfig       *model.Config
		readErr          error
		configApplierErr error
		expectedStatus   int
		expectedError    string
		// expectCheck is whether the reloaded configuration reaches the preflight check.
		// A reload that is rejected must not: it would dial clusters and write probe
		// objects into buckets on behalf of a configuration that never takes effect.
		expectCheck bool
	}{
		{
			name:           "read failure",
			readErr:        errors.New("read boom"),
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "read boom",
		},
		{
			name: "static field changed",
			readConfig: func() *model.Config {
				c := model.NewConfig()
				c.ServiceConfig.ServerHTTP = &model.ServerConfigHTTP{Port: ptr.Of(model.Port(9999))}
				return c
			}(),
			expectedStatus: http.StatusBadRequest,
			expectedError:  "static configuration has changed",
		},
		{
			// The running configuration is the old side of the comparison and the file is
			// the new one. With the two the other way round, this reads "removed".
			name: "static field added by the file is reported as added",
			readConfig: func() *model.Config {
				c := model.NewConfig()
				c.ServiceConfig.ServerHTTPS = &model.ServerConfigHTTPS{Port: ptr.Of(model.Port(8443))}
				return c
			}(),
			expectedStatus: http.StatusBadRequest,
			expectedError:  "ServerHTTPS added",
		},
		{
			// The reload was accepted and installed; only scheduling it failed.
			name:             "apply failure",
			readConfig:       model.NewConfig(),
			configApplierErr: errors.New("apply boom"),
			expectedStatus:   http.StatusInternalServerError,
			expectedError:    "apply boom",
			expectCheck:      true,
		},
		{
			name:           "success",
			readConfig:     model.NewConfig(),
			expectedStatus: http.StatusOK,
			expectCheck:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, ctrl := newConfigTestService(t)

			mockConfigurationManager := configuration.NewMockManager(ctrl)
			mockConfigurationManager.EXPECT().Read(gomock.Any()).Return(tt.readConfig, tt.readErr)
			svc.configurationManager = mockConfigurationManager

			mockConfigApplier := service.NewMockConfigApplier(ctrl)
			if tt.expectedStatus == http.StatusOK || tt.configApplierErr != nil {
				mockConfigApplier.EXPECT().ApplyNewConfig().Return(tt.configApplierErr)
			}
			svc.configApplier = mockConfigApplier

			// The check runs on its own goroutine, so a call is awaited rather than
			// asserted straight after the handler returns.
			checked := make(chan struct{}, 1)
			checker := preflight.NewMockChecker(ctrl)
			if tt.expectCheck {
				checker.EXPECT().
					Check(gomock.Any(), gomock.Any()).
					Do(func(context.Context, *model.BackupConfig) { close(checked) }).
					Times(1)
			}
			svc.checker = checker

			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/config/apply", nil)
			w := httptest.NewRecorder()

			svc.ApplyConfig(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}

			if tt.expectCheck {
				select {
				case <-checked:
				case <-time.After(5 * time.Second):
					t.Fatal("the accepted reload was never handed to the checker")
				}
			}
		})
	}
}

func TestService_changeConfig(t *testing.T) {
	svc, ctrl := newConfigTestService(t)

	mockConfigurationManager := configuration.NewMockManager(ctrl)
	mockConfigurationManager.EXPECT().Write(gomock.Any(), gomock.Any()).Return(nil)
	svc.configurationManager = mockConfigurationManager

	mockConfigApplier := service.NewMockConfigApplier(ctrl)
	mockConfigApplier.EXPECT().ApplyNewConfig().Return(nil)
	svc.configApplier = mockConfigApplier

	called := false
	err := svc.changeConfig(t.Context(), func(config *model.Config) error {
		called = true
		return nil
	})

	require.NoError(t, err)
	assert.True(t, called)
}

func TestService_UpdateConfig_PreservesSecretOnRoundTrip(t *testing.T) {
	const realPassword = "real-secret-password"

	svc, ctrl := newConfigTestService(t)
	clusterModel := &model.AerospikeCluster{
		SeedNodes: []model.SeedNode{{HostName: "localhost", Port: 3000}},
		Credentials: &model.Credentials{
			User:     "testUser",
			Password: realPassword,
			AuthMode: model.AuthModeInternal,
		},
	}
	require.NoError(t, svc.config.AddCluster("test-cluster", clusterModel))

	mockConfigurationManager := configuration.NewMockManager(ctrl)
	mockConfigurationManager.EXPECT().Write(gomock.Any(), gomock.Any()).Return(nil)
	svc.configurationManager = mockConfigurationManager

	mockConfigApplier := service.NewMockConfigApplier(ctrl)
	mockConfigApplier.EXPECT().ApplyNewConfig().Return(nil)
	svc.configApplier = mockConfigApplier

	getReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/config", nil)
	getW := httptest.NewRecorder()
	svc.ReadConfig(getW, getReq)
	require.Equal(t, http.StatusOK, getW.Code)

	var configDTO dto.Config
	require.NoError(t, json.NewDecoder(getW.Body).Decode(&configDTO))
	require.NotNil(t, configDTO.AerospikeClusters["test-cluster"].Credentials)
	assert.Equal(t, "[secret]", string(configDTO.AerospikeClusters["test-cluster"].Credentials.Password))

	configDTO.AerospikeClusters["test-cluster"].ClusterLabel = "updated-label"
	putBody, err := json.Marshal(configDTO)
	require.NoError(t, err)

	putReq := httptest.NewRequestWithContext(
		t.Context(), http.MethodPut, "/v1/config", strings.NewReader(string(putBody)),
	)
	putW := httptest.NewRecorder()
	svc.UpdateConfig(putW, putReq)
	require.Equal(t, http.StatusOK, putW.Code)

	updated, ok := svc.config.BackupConfigCopy().AerospikeClusters["test-cluster"]
	require.True(t, ok)
	require.NotNil(t, updated.Credentials)
	assert.Equal(t, redact.Secret(realPassword), updated.Credentials.Password)
	assert.Equal(t, "updated-label", updated.ClusterLabel)
}

func TestService_changeConfig_UpdateFuncError(t *testing.T) {
	svc, _ := newConfigTestService(t)

	err := svc.changeConfig(t.Context(), func(config *model.Config) error {
		return errors.New("update boom")
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "update boom")
}
