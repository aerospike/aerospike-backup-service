package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddPolicy(t *testing.T) {
	tests := []struct {
		name           string
		policyName     string
		requestBody    string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "successful add",
			policyName:     "test-policy",
			requestBody:    marshalToString(dto.BackupPolicy{Parallel: ptr.Of(8)}),
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "missing policy name",
			policyName:     "",
			requestBody:    "{}",
			expectedStatus: http.StatusBadRequest,
			expectedError:  errMissingPolicyName.Error(),
		},
		{
			name:           "invalid json",
			policyName:     "test-policy",
			requestBody:    "{noField : 1}",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid JSON payload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := setupTestService()

			req := httptest.NewRequest(http.MethodPost, "/v1/config/policies/"+tt.policyName, strings.NewReader(tt.requestBody))
			req.SetPathValue("name", tt.policyName)
			w := httptest.NewRecorder()

			svc.AddPolicy(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}

func TestReadPolicies(t *testing.T) {
	svc := setupTestService()
	svc.config = model.NewConfig()
	_ = svc.config.AddPolicy("policy1", &model.BackupPolicy{})
	_ = svc.config.AddPolicy("policy2", &model.BackupPolicy{})

	req := httptest.NewRequest(http.MethodGet, "/v1/config/policies", nil)
	w := httptest.NewRecorder()

	svc.ReadPolicies(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]dto.BackupPolicy
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Len(t, response, 2)
	assert.Contains(t, response, "policy1")
	assert.Contains(t, response, "policy2")
}

//nolint:dupl
func TestReadPolicy(t *testing.T) {
	tests := []struct {
		name           string
		policyName     string
		policy         *model.BackupPolicy
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "existing policy",
			policyName:     "test-policy",
			policy:         &model.BackupPolicy{},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing policy name",
			policyName:     "",
			expectedStatus: http.StatusBadRequest,
			expectedError:  errMissingPolicyName.Error(),
		},
		{
			name:           "non-existent policy",
			policyName:     "non-existent",
			expectedStatus: http.StatusNotFound,
			expectedError:  errNotFound("policy", "non-existent").Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := setupTestService()
			if tt.policy != nil {
				_ = svc.config.AddPolicy(tt.policyName, tt.policy)
			}

			req := httptest.NewRequest(http.MethodGet, "/v1/config/policies/"+tt.policyName, nil)
			req.SetPathValue("name", tt.policyName)
			w := httptest.NewRecorder()

			svc.ReadPolicy(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}

func TestUpdatePolicy(t *testing.T) {
	tests := []struct {
		name           string
		policyName     string
		requestBody    string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "successful update",
			policyName:     "test-policy",
			requestBody:    "{}",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing policy name",
			policyName:     "",
			requestBody:    "{}",
			expectedStatus: http.StatusBadRequest,
			expectedError:  errMissingPolicyName.Error(),
		},
		{
			name:           "invalid json",
			policyName:     "test-policy",
			requestBody:    "{nil}",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid JSON payload",
		},
		{
			name:           "unknown policy name",
			policyName:     "unknown-policy",
			requestBody:    "{}",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := setupTestService()
			_ = svc.config.AddPolicy("test-policy", &model.BackupPolicy{})

			req := httptest.NewRequest(http.MethodPut, "/v1/config/policies/"+tt.policyName, strings.NewReader(tt.requestBody))
			req.SetPathValue("name", tt.policyName)
			w := httptest.NewRecorder()

			svc.UpdatePolicy(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}

//nolint:dupl
func TestDeletePolicy(t *testing.T) {
	tests := []struct {
		name           string
		policyName     string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "successful delete",
			policyName:     "test-policy",
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "missing policy name",
			policyName:     "",
			expectedStatus: http.StatusBadRequest,
			expectedError:  errMissingPolicyName.Error(),
		},
		{
			name:           "unknown policy name",
			policyName:     "unknown-policy",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := setupTestService()
			_ = svc.config.AddPolicy("test-policy", &model.BackupPolicy{})

			req := httptest.NewRequest(http.MethodDelete, "/v1/config/policies/"+tt.policyName, nil)
			req.SetPathValue("name", tt.policyName)

			w := httptest.NewRecorder()

			svc.DeletePolicy(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}

// Helper function to setup test service with mocked dependencies.
func setupTestService() *Service {
	mockScheduler := &MockScheduler{}
	mockConfigApplier := &MockConfigApplier{}
	mockConfigurationManager := &configurationManagerMock{}
	mockRegistry := &mockRunningBackupsRegistry{}

	return NewService(
		context.Background(),
		model.NewConfig(),
		mockConfigApplier,
		mockScheduler,
		nil,
		nil,
		nil,
		mockRegistry,
		mockConfigurationManager,
		nil,
	)
}
