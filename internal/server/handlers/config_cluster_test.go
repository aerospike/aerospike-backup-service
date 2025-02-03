package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
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

			req := httptest.NewRequest(http.MethodPost, "/v1/config/clusters/"+tt.clusterName, strings.NewReader(tt.requestBody))
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
	svc.config = model.NewConfig()

	_ = svc.config.AddCluster("cluster1", &model.AerospikeCluster{})
	_ = svc.config.AddCluster("cluster2", &model.AerospikeCluster{})

	req := httptest.NewRequest(http.MethodGet, "/v1/config/clusters", nil)
	w := httptest.NewRecorder()

	svc.ReadAerospikeClusters(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]dto.AerospikeCluster
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
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
			if tt.cluster != nil {
				_ = svc.config.AddCluster(tt.clusterName, tt.cluster)
			}

			req := httptest.NewRequest(http.MethodGet, "/v1/config/clusters/"+tt.clusterName, nil)
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
		name               string
		clusterName        string
		requestBody        string
		mockValidatorError error
		expectedStatus     int
		expectedError      string
	}{
		{
			name:               "successful update",
			clusterName:        "test-cluster",
			requestBody:        marshalToString(cluster),
			mockValidatorError: nil,
			expectedStatus:     http.StatusOK,
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
		{
			name:               "namespace validation failure",
			clusterName:        "test-cluster",
			requestBody:        marshalToString(cluster),
			mockValidatorError: fmt.Errorf("invalid namespace"),
			expectedStatus:     http.StatusBadRequest,
			expectedError:      "invalid namespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := setupTestService()
			if tt.mockValidatorError != nil {
				svc.nsValidator = &mockNamespaceValidator{
					validateError: tt.mockValidatorError,
				}
			}
			initialCluster := &model.AerospikeCluster{}
			_ = svc.config.AddCluster("test-cluster", initialCluster)

			req := httptest.NewRequest(http.MethodPut, "/v1/config/clusters/"+tt.clusterName, strings.NewReader(tt.requestBody))
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
			_ = svc.config.AddCluster("test-cluster", &model.AerospikeCluster{})

			req := httptest.NewRequest(http.MethodDelete, "/v1/config/clusters/"+tt.clusterName, nil)
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
