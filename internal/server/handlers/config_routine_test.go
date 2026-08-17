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
	"go.uber.org/mock/gomock"
)

func TestAddRoutine(t *testing.T) {
	tests := []struct {
		name           string
		routineName    string
		requestBody    string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "missing routine name",
			routineName:    "",
			requestBody:    "{}",
			expectedStatus: http.StatusBadRequest,
			expectedError:  errMissingRoutineName.Error(),
		},
		{
			name:           "invalid json",
			routineName:    "test-routine",
			requestBody:    "{noField : 1}",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid JSON payload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := setupTestService(t)
			_ = svc.config.AddPolicy("test-policy", &model.BackupPolicy{})

			req := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"/v1/config/routines/"+tt.routineName,
				strings.NewReader(tt.requestBody),
			)
			req.SetPathValue("name", tt.routineName)
			w := httptest.NewRecorder()

			svc.AddRoutine(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}

func TestReadRoutines(t *testing.T) {
	svc := setupTestService(t)
	svc.config = model.NewConfig()
	_ = svc.config.AddRoutine(&model.BackupRoutine{Name: "routine1"})
	_ = svc.config.AddRoutine(&model.BackupRoutine{Name: "routine2"})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/config/routines", nil)
	w := httptest.NewRecorder()

	svc.ReadRoutines(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]dto.BackupRoutine
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Len(t, response, 2)
	assert.Contains(t, response, "routine1")
	assert.Contains(t, response, "routine2")
}

func TestReadRoutine(t *testing.T) {
	tests := []struct {
		name           string
		routineName    string
		routine        *model.BackupRoutine
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "existing routine",
			routineName:    "test-routine",
			routine:        &model.BackupRoutine{Name: "test-routine"},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing routine name",
			expectedStatus: http.StatusBadRequest,
			expectedError:  errMissingRoutineName.Error(),
		},
		{
			name:           "non-existent routine",
			routineName:    "non-existent",
			expectedStatus: http.StatusNotFound,
			expectedError:  errRoutineNotFound("non-existent").Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := setupTestService(t)
			if tt.routine != nil {
				_ = svc.config.AddRoutine(tt.routine)
			}

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/config/routines/"+tt.routineName, nil)
			req.SetPathValue("name", tt.routineName)
			w := httptest.NewRecorder()

			svc.ReadRoutine(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}

func TestUpdateRoutine(t *testing.T) {
	tests := []struct {
		name           string
		routineName    string
		requestBody    string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "missing routine name",
			routineName:    "",
			requestBody:    "{}",
			expectedStatus: http.StatusBadRequest,
			expectedError:  errMissingRoutineName.Error(),
		},
		{
			name:           "invalid json",
			routineName:    "test-routine",
			requestBody:    "{nil}",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid JSON payload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := setupTestService(t)

			req := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPut,
				"/v1/config/routines/"+tt.routineName,
				strings.NewReader(tt.requestBody),
			)
			req.SetPathValue("name", tt.routineName)
			w := httptest.NewRecorder()

			svc.UpdateRoutine(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}

func TestDeleteRoutine(t *testing.T) {
	tests := []struct {
		name           string
		routineName    string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "successful delete",
			routineName:    "test-routine",
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "missing routine name",
			routineName:    "",
			expectedStatus: http.StatusBadRequest,
			expectedError:  errMissingRoutineName.Error(),
		},
		{
			name:           "unknown routine name",
			routineName:    "unknown-routine",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := setupTestService(t)
			_ = svc.config.AddRoutine(&model.BackupRoutine{Name: "test-routine"})

			req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/v1/config/routines/"+tt.routineName, nil)
			req.SetPathValue("name", tt.routineName)
			w := httptest.NewRecorder()

			svc.DeleteRoutine(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}

func TestEnableRoutine(t *testing.T) {
	tests := []struct {
		name           string
		routineName    string
		expectedStatus int
		expectedError  string
		addRoutine     bool
	}{
		{
			name:           "successful enable",
			routineName:    "test-routine",
			addRoutine:     true,
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "missing routine name",
			routineName:    "",
			expectedStatus: http.StatusBadRequest,
			addRoutine:     true,
			expectedError:  errMissingRoutineName.Error(),
		},
		{
			name:           "non-existent routine",
			routineName:    "unknown-routine",
			expectedStatus: http.StatusNotFound,
			expectedError:  errRoutineNotFound("unknown-routine").Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := setupTestService(t)
			if tt.addRoutine {
				addValidBackupRoutine(svc, tt.routineName, true)
			}

			req := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPut,
				"/v1/config/routines/"+tt.routineName+"/enable",
				nil,
			)
			req.SetPathValue("name", tt.routineName)
			w := httptest.NewRecorder()

			svc.EnableRoutine(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			} else {
				updated, ok := svc.config.Routine(tt.routineName)
				require.True(t, ok)
				assert.False(t, updated.Disabled)
			}
		})
	}
}

func TestDisableRoutine(t *testing.T) {
	tests := []struct {
		name               string
		routineName        string
		expectedStatus     int
		expectedError      string
		expectedCancelRuns int
	}{
		{
			name:               "successful disable",
			routineName:        "test-routine",
			expectedStatus:     http.StatusNoContent,
			expectedCancelRuns: 1,
		},
		{
			name:           "missing routine name",
			routineName:    "",
			expectedStatus: http.StatusBadRequest,
			expectedError:  errMissingRoutineName.Error(),
		},
		{
			name:           "non-existent routine",
			routineName:    "unknown-routine",
			expectedStatus: http.StatusNotFound,
			expectedError:  errRoutineNotFound("unknown-routine").Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRegistry := NewmockRunningBackupsRegistry(ctrl)
			mockRegistry.EXPECT().Cancel(tt.routineName).Times(tt.expectedCancelRuns)

			svc := setupTestService(t)
			svc.registry = mockRegistry
			addValidBackupRoutine(svc, "test-routine", false)

			req := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPut,
				"/v1/config/routines/"+tt.routineName+"/disable",
				nil,
			)
			req.SetPathValue("name", tt.routineName)
			w := httptest.NewRecorder()

			svc.DisableRoutine(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			} else {
				updated, ok := svc.config.Routine(tt.routineName)
				require.True(t, ok)
				assert.True(t, updated.Disabled)
			}
		})
	}
}
