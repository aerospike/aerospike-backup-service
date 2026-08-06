package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var cluster = dto.AerospikeCluster{SeedNodes: []dto.SeedNode{{HostName: "localhost", Port: 3000}}}

func TestAddAerospikeCluster(t *testing.T) {
	tests := []struct {
		name           string
		clusterName    string
		requestBody    string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "successful add",
			clusterName:    "test-cluster",
			requestBody:    marshalToString(cluster),
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "missing cluster name",
			clusterName:    "",
			requestBody:    "{}",
			expectedStatus: http.StatusBadRequest,
			expectedError:  errMissingClusterName.Error(),
		},
		{
			name:           "invalid json",
			clusterName:    "test-cluster",
			requestBody:    "{noField : 1}",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid JSON payload",
		},
		{
			name:           "invalid cluster config",
			clusterName:    "test-cluster",
			requestBody:    marshalToString(dto.AerospikeCluster{}),
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid JSON payload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := setupTestService()

			req := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"/v1/config/clusters/"+tt.clusterName,
				strings.NewReader(tt.requestBody),
			)
			req.SetPathValue("name", tt.clusterName)
			w := httptest.NewRecorder()

			svc.AddAerospikeCluster(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}

func TestReadAerospikeClusters(t *testing.T) {
	svc := setupTestService()
	configManager := svc.configManager.(*ConfigManagerImpl)

	_ = configManager.config.AddCluster("cluster1", &model.AerospikeCluster{})
	_ = configManager.config.AddCluster("cluster2", &model.AerospikeCluster{})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/config/clusters", nil)
	w := httptest.NewRecorder()

	svc.ReadAerospikeClusters(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]dto.AerospikeCluster
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Len(t, response, 2)
	assert.Contains(t, response, "cluster1")
	assert.Contains(t, response, "cluster2")
}

//nolint:dupl
func TestReadAerospikeCluster(t *testing.T) {
	tests := []struct {
		name           string
		clusterName    string
		cluster        *model.AerospikeCluster
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "existing cluster",
			clusterName:    "test-cluster",
			cluster:        &model.AerospikeCluster{},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing cluster name",
			clusterName:    "",
			expectedStatus: http.StatusBadRequest,
			expectedError:  errMissingClusterName.Error(),
		},
		{
			name:           "non-existent cluster",
			clusterName:    "non-existent",
			expectedStatus: http.StatusNotFound,
			expectedError:  errNotFound("cluster", "non-existent").Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := setupTestService()
			configManager := svc.configManager.(*ConfigManagerImpl)
			if tt.cluster != nil {
				_ = configManager.config.AddCluster(tt.clusterName, tt.cluster)
			}

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/config/clusters/"+tt.clusterName, nil)
			req.SetPathValue("name", tt.clusterName)
			w := httptest.NewRecorder()

			svc.ReadAerospikeCluster(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}

func TestUpdateAerospikeCluster(t *testing.T) {
	tests := []struct {
		name           string
		clusterName    string
		requestBody    string
		expectedStatus int
		expectedError  string
		runValidation  bool
	}{
		{
			name:           "successful update",
			clusterName:    "test-cluster",
			requestBody:    marshalToString(cluster),
			expectedStatus: http.StatusOK,
			runValidation:  true,
		},
		{
			name:           "missing cluster name",
			clusterName:    "",
			requestBody:    "{}",
			expectedStatus: http.StatusBadRequest,
			expectedError:  errMissingClusterName.Error(),
		},
		{
			name:           "invalid json",
			clusterName:    "test-cluster",
			requestBody:    "{nil}",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid JSON payload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockNsValidator := aerospike.NewMockNamespaceValidator(ctrl)
			svc := setupTestService(mockNsValidator)
			configManager := svc.configManager.(*ConfigManagerImpl)

			if tt.runValidation {
				mockNsValidator.EXPECT().Validate(gomock.Any(), gomock.Eq(configManager.config))
			}

			initialCluster := &model.AerospikeCluster{}
			_ = configManager.config.AddCluster("test-cluster", initialCluster)

			req := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPut,
				"/v1/config/clusters/"+tt.clusterName,
				strings.NewReader(tt.requestBody),
			)
			req.SetPathValue("name", tt.clusterName)
			w := httptest.NewRecorder()

			svc.UpdateAerospikeCluster(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}

//nolint:dupl
func TestDeleteAerospikeCluster(t *testing.T) {
	tests := []struct {
		name           string
		clusterName    string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "successful delete",
			clusterName:    "test-cluster",
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "missing cluster name",
			clusterName:    "",
			expectedStatus: http.StatusBadRequest,
			expectedError:  errMissingClusterName.Error(),
		},
		{
			name:           "unknown cluster name",
			clusterName:    "unknown-cluster",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := setupTestService()
			configManager := svc.configManager.(*ConfigManagerImpl)
			_ = configManager.config.AddCluster("test-cluster", &model.AerospikeCluster{})

			req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/v1/config/clusters/"+tt.clusterName, nil)
			req.SetPathValue("name", tt.clusterName)
			w := httptest.NewRecorder()

			svc.DeleteAerospikeCluster(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}

func TestDeleteAerospikeCluster_InUseErrorMessage(t *testing.T) {
	svc := setupTestService()
	entities := addValidBackupConfig(svc)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/v1/config/clusters/"+entities.clusterName, nil)
	req.SetPathValue("name", entities.clusterName)
	w := httptest.NewRecorder()

	svc.DeleteAerospikeCluster(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(),
		"delete Aerospike cluster \"cluster1\": item is in use: it is used in routine \"routine1\"")
}
