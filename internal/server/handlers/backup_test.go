package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_GetAllFullBackups(t *testing.T) {
	tests := []struct {
		name           string
		queryParams    map[string]string
		setupMock      func(*MockBackendsService)
		expectedStatus int
		expectedBody   map[string][]dto.BackupDetails
	}{
		{
			name: "successful retrieval",
			queryParams: map[string]string{
				"from": "1000",
				"to":   "2000",
			},
			setupMock: func(m *MockBackendsService) {
				m.f = func(_ context.Context, _ service.BackupFilter) ([]model.BackupDetails, error) {
					return []model.BackupDetails{{Key: "backup1", Routine: "routine1"}}, nil
				}
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string][]dto.BackupDetails{
				"routine1": {
					{
						Key: "backup1",
					},
				},
			},
		},
		{
			name: "invalid time bounds",
			queryParams: map[string]string{
				"from": "invalid",
				"to":   "2000",
			},
			setupMock:      func(*MockBackendsService) {},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockBackends := &MockBackendsService{}
			cfg := model.NewConfig()

			tt.setupMock(mockBackends)

			svc := &Service{
				config:         cfg,
				backendService: mockBackends,
			}

			req := httptest.NewRequest(http.MethodGet, "/v1/backups/full", nil)
			q := req.URL.Query()
			for k, v := range tt.queryParams {
				q.Add(k, v)
			}
			req.URL.RawQuery = q.Encode()

			w := httptest.NewRecorder()
			svc.GetAllFullBackups(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var response map[string][]dto.BackupDetails
				err := json.NewDecoder(w.Body).Decode(&response)
				require.NoError(t, err)
				assert.Equal(t, tt.expectedBody, response)
			}
		})
	}
}

func TestService_ScheduleFullBackup(t *testing.T) {
	tests := []struct {
		name           string
		routineName    string
		delayParam     string
		expectedStatus int
	}{
		{
			name:           "invalid delay parameter",
			routineName:    "test-routine",
			delayParam:     "invalid",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "negative delay",
			routineName:    "test-routine",
			delayParam:     "-1000",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "empty routine name",
			routineName:    "",
			delayParam:     "1000",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockScheduler := &MockScheduler{}
			cfg := model.NewConfig()

			svc := &Service{
				config:    cfg,
				scheduler: mockScheduler,
			}

			req := httptest.NewRequest(http.MethodPost, "/v1/backups/schedule/"+tt.routineName, nil)
			if tt.delayParam != "" {
				q := req.URL.Query()
				q.Add("delay", tt.delayParam)
				req.URL.RawQuery = q.Encode()
			}
			req.SetPathValue("name", tt.routineName)

			w := httptest.NewRecorder()
			svc.ScheduleFullBackup(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockScheduler.AssertExpectations(t)
		})
	}
}

func TestService_CancelCurrentBackup(t *testing.T) {
	svc := &Service{}

	req := httptest.NewRequest(http.MethodPost, "/v1/backups/cancel/", nil)
	w := httptest.NewRecorder()
	svc.CancelCurrentBackup(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
