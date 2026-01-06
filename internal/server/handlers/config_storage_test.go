package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddStorage(t *testing.T) {
	tests := []struct {
		name           string
		storageName    string
		requestBody    string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "successful add",
			storageName:    "test-storage",
			requestBody:    marshalToString(dto.Storage{LocalStorage: &dto.LocalStorage{Path: "/"}}),
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "missing storage name",
			storageName:    "",
			requestBody:    "{}",
			expectedStatus: http.StatusBadRequest,
			expectedError:  errMissingStorageName.Error(),
		},
		{
			name:           "invalid json",
			storageName:    "test-storage",
			requestBody:    "{noField : 1}",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid JSON payload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := setupTestService()

			req := httptest.NewRequest(http.MethodPost, "/v1/config/storage/"+tt.storageName, strings.NewReader(tt.requestBody))
			req.SetPathValue("name", tt.storageName)
			w := httptest.NewRecorder()

			svc.AddStorage(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}

func TestReadAllStorage(t *testing.T) {
	svc := setupTestService()
	svc.config = model.NewConfig()

	_ = svc.config.AddStorage("storage1", &model.LocalStorage{})
	_ = svc.config.AddStorage("storage2", &model.LocalStorage{})

	req := httptest.NewRequest(http.MethodGet, "/v1/config/storage", nil)
	w := httptest.NewRecorder()

	svc.ReadAllStorage(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]dto.Storage
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Len(t, response, 2)
	assert.Contains(t, response, "storage1")
	assert.Contains(t, response, "storage2")
}

func TestReadStorage(t *testing.T) {
	tests := []struct {
		name           string
		storageName    string
		storage        model.Storage
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "existing storage",
			storageName:    "test-storage",
			storage:        &model.LocalStorage{},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing storage name",
			storageName:    "",
			expectedStatus: http.StatusBadRequest,
			expectedError:  errMissingStorageName.Error(),
		},
		{
			name:           "non-existent storage",
			storageName:    "non-existent",
			expectedStatus: http.StatusNotFound,
			expectedError:  errNotFound("storage", "non-existent").Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := setupTestService()
			if tt.storage != nil {
				_ = svc.config.AddStorage(tt.storageName, tt.storage)
			}

			req := httptest.NewRequest(http.MethodGet, "/v1/config/storage/"+tt.storageName, nil)
			req.SetPathValue("name", tt.storageName)
			w := httptest.NewRecorder()

			svc.ReadStorage(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}

func TestUpdateStorage(t *testing.T) {
	tests := []struct {
		name           string
		storageName    string
		requestBody    string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "successful update",
			storageName:    "test-storage",
			requestBody:    marshalToString(dto.Storage{LocalStorage: &dto.LocalStorage{Path: "/tmp"}}),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing storage name",
			storageName:    "",
			requestBody:    "{}",
			expectedStatus: http.StatusBadRequest,
			expectedError:  errMissingStorageName.Error(),
		},
		{
			name:           "invalid json",
			storageName:    "test-storage",
			requestBody:    "{nil}",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid JSON payload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := setupTestService()
			initialStorage := &model.LocalStorage{Path: "/"}
			_ = svc.config.AddStorage("test-storage", initialStorage)

			req := httptest.NewRequest(http.MethodPut, "/v1/config/storage/"+tt.storageName, strings.NewReader(tt.requestBody))
			req.SetPathValue("name", tt.storageName)
			w := httptest.NewRecorder()

			svc.UpdateStorage(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}

//nolint:dupl
func TestDeleteStorage(t *testing.T) {
	tests := []struct {
		name           string
		storageName    string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "successful delete",
			storageName:    "test-storage",
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "missing storage name",
			storageName:    "",
			expectedStatus: http.StatusBadRequest,
			expectedError:  errMissingStorageName.Error(),
		},
		{
			name:           "unknown storage name",
			storageName:    "unknown-storage",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := setupTestService()
			_ = svc.config.AddStorage("test-storage", &model.LocalStorage{})

			req := httptest.NewRequest(http.MethodDelete, "/v1/config/storage/"+tt.storageName, nil)
			req.SetPathValue("name", tt.storageName)
			w := httptest.NewRecorder()

			svc.DeleteStorage(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}

func marshalToString(obj any) string {
	x, _ := json.Marshal(obj)
	return string(x)
}
