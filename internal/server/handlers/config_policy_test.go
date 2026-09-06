package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/internal/server/configuration"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
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
			svc := setupTestService(t)

			req := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"/v1/config/policies/"+tt.policyName,
				strings.NewReader(tt.requestBody),
			)
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
	svc := setupTestService(t)
	svc.config = model.NewConfig()
	_ = svc.config.AddPolicy("policy1", &model.BackupPolicy{})
	_ = svc.config.AddPolicy("policy2", &model.BackupPolicy{})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/config/policies", nil)
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
			svc := setupTestService(t)
			if tt.policy != nil {
				_ = svc.config.AddPolicy(tt.policyName, tt.policy)
			}

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/config/policies/"+tt.policyName, nil)
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
		name             string
		policyName       string
		requestBody      string
		expectedStatus   int
		expectedError    string
		maxParallelScans *int
	}{
		{
			name:             "parallel exceeds cluster max for routine using policy",
			policyName:       "test-policy",
			requestBody:      marshalToString(dto.BackupPolicy{Parallel: ptr.Of(20)}),
			expectedStatus:   http.StatusBadRequest,
			expectedError:    "backup policy parallelism 20 exceeds cluster max parallelism 10",
			maxParallelScans: ptr.Of(10),
		},
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
			svc := setupTestService(t)
			entities := addValidBackupConfig(svc)
			if tt.maxParallelScans != nil {
				entities.cluster.MaxParallelScans = tt.maxParallelScans
			}

			req := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPut,
				"/v1/config/policies/"+tt.policyName,
				strings.NewReader(tt.requestBody),
			)
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
			svc := setupTestService(t)
			_ = svc.config.AddPolicy("test-policy", &model.BackupPolicy{})

			req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/v1/config/policies/"+tt.policyName, nil)
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

func TestDeletePolicy_InUseErrorMessage(t *testing.T) {
	svc := setupTestService(t)
	entities := addValidBackupConfig(svc)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/v1/config/policies/"+entities.policyName, nil)
	req.SetPathValue("name", entities.policyName)
	w := httptest.NewRecorder()

	svc.DeletePolicy(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(),
		"delete backup policy \"test-policy\": item is in use: it is used in routine \"routine1\"")
}

func TestUpdatePolicy_Case2_ClusterMaxSetBeforeParallelIncrease(t *testing.T) {
	ctrl := gomock.NewController(t)

	svc := setupTestService(t)
	mockNsValidator := aerospike.NewMockNamespaceValidator(ctrl)
	svc.nsValidator = mockNsValidator
	mockNsValidator.EXPECT().Validate(gomock.Any(), gomock.Any()).Times(1)

	entities := addValidBackupConfig(svc)
	entities.policy.Parallel = ptr.Of(1)

	// Step 1: introduce max-parallel-scans=10 on the cluster.
	clusterReq := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPut,
		"/v1/config/clusters/"+entities.clusterName,
		strings.NewReader(marshalToString(dto.AerospikeCluster{
			SeedNodes:        []dto.SeedNode{{HostName: "localhost", Port: 3000}},
			MaxParallelScans: ptr.Of(10),
		})),
	)
	clusterReq.SetPathValue("name", entities.clusterName)
	clusterW := httptest.NewRecorder()
	svc.UpdateAerospikeCluster(clusterW, clusterReq)
	require.Equal(t, http.StatusOK, clusterW.Code)

	// Step 2: raise parallel above the new cluster max — should be rejected.
	policyReq := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPut,
		"/v1/config/policies/"+entities.policyName,
		strings.NewReader(marshalToString(dto.BackupPolicy{Parallel: ptr.Of(20)})),
	)
	policyReq.SetPathValue("name", entities.policyName)
	policyW := httptest.NewRecorder()
	svc.UpdatePolicy(policyW, policyReq)

	assert.Equal(t, http.StatusBadRequest, policyW.Code)
	assert.Contains(t, policyW.Body.String(), "backup policy parallelism 20 exceeds cluster max parallelism 10")
}

// Helper function to setup test service with mocked dependencies.
func setupTestService(t *testing.T) *Service {
	t.Helper()

	ctrl := gomock.NewController(t)
	mockManager := configuration.NewMockManager(ctrl)
	mockManager.EXPECT().Write(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	mockConfigApplier := service.NewMockConfigApplier(ctrl)
	mockConfigApplier.EXPECT().ApplyNewConfig(gomock.Any()).Return(nil).AnyTimes()

	return NewService(
		t.Context(),
		model.NewConfig(),
		mockConfigApplier,
		nil,
		nil,
		nil,
		nil,
		nil,
		mockManager,
		nil,
		newMockTLSProber(ctrl),
	)
}
