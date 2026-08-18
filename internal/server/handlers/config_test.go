package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
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
	mockNsValidator := aerospike.NewMockNamespaceValidator(ctrl)
	mockNsValidator.EXPECT().Validate(gomock.Any(), gomock.Any()).AnyTimes()

	return &Service{
		sysCtx:      t.Context(),
		config:      model.NewConfig(),
		nsValidator: mockNsValidator,
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
			name:           "invalid json payload",
			requestBody:    "{noField : 1}",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid JSON payload",
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

			mockConfigurationManager := NewmockManager(ctrl)
			if tt.expectedStatus != http.StatusBadRequest {
				mockConfigurationManager.EXPECT().Write(gomock.Any(), gomock.Any()).Return(tt.writeErr)
			}
			svc.configurationManager = mockConfigurationManager

			mockConfigApplier := NewmockConfigApplier(ctrl)
			if tt.expectedStatus == http.StatusOK || tt.configApplierErr != nil {
				mockConfigApplier.EXPECT().ApplyNewConfig(gomock.Any()).Return(tt.configApplierErr)
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
				c.ServiceConfig.HTTPServer = &model.HTTPServerConfig{Port: ptr.Of(model.Port(9999))}
				return c
			}(),
			expectedStatus: http.StatusBadRequest,
			expectedError:  "static configuration has changed",
		},
		{
			name:             "apply failure",
			readConfig:       model.NewConfig(),
			configApplierErr: errors.New("apply boom"),
			expectedStatus:   http.StatusInternalServerError,
			expectedError:    "apply boom",
		},
		{
			name:           "success",
			readConfig:     model.NewConfig(),
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, ctrl := newConfigTestService(t)

			mockConfigurationManager := NewmockManager(ctrl)
			mockConfigurationManager.EXPECT().Read(gomock.Any()).Return(tt.readConfig, tt.readErr)
			svc.configurationManager = mockConfigurationManager

			mockConfigApplier := NewmockConfigApplier(ctrl)
			if tt.expectedStatus == http.StatusOK || tt.configApplierErr != nil {
				mockConfigApplier.EXPECT().ApplyNewConfig(gomock.Any()).Return(tt.configApplierErr)
			}
			svc.configApplier = mockConfigApplier

			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/config/apply", nil)
			w := httptest.NewRecorder()

			svc.ApplyConfig(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}

func TestService_changeConfig(t *testing.T) {
	svc, ctrl := newConfigTestService(t)

	mockConfigurationManager := NewmockManager(ctrl)
	mockConfigurationManager.EXPECT().Write(gomock.Any(), gomock.Any()).Return(nil)
	svc.configurationManager = mockConfigurationManager

	mockConfigApplier := NewmockConfigApplier(ctrl)
	mockConfigApplier.EXPECT().ApplyNewConfig(gomock.Any()).Return(nil)
	svc.configApplier = mockConfigApplier

	called := false
	err := svc.changeConfig(t.Context(), func(config *model.Config) error {
		called = true
		return nil
	})

	require.NoError(t, err)
	assert.True(t, called)
}

func TestService_changeConfig_UpdateFuncError(t *testing.T) {
	svc, _ := newConfigTestService(t)

	err := svc.changeConfig(t.Context(), func(config *model.Config) error {
		return errors.New("update boom")
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "update boom")
}
