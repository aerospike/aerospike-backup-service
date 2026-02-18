package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestService_GetAllFullBackups(t *testing.T) {
	tests := []struct {
		name           string
		queryParams    map[string]string
		setupMock      func(*MockBackupBackendService)
		expectedStatus int
		expectedBody   map[string][]dto.BackupDetails
	}{
		{
			name: "successful retrieval",
			queryParams: map[string]string{
				"from": "1000",
				"to":   "2000",
			},
			setupMock: func(m *MockBackupBackendService) {
				m.On("GetBackups", mock.Anything, mock.Anything).
					Return([]model.BackupDetails{{Key: "backup1", BackupMetadata: model.BackupMetadata{
						Created:  time.UnixMilli(1000).In(time.UTC),
						Finished: time.UnixMilli(5000).In(time.UTC),
					}}}, nil)
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string][]dto.BackupDetails{
				"routine1": {
					{
						Key:       "backup1",
						Timestamp: 1000,
						Created:   time.UnixMilli(1000).In(time.UTC),
						Finished:  time.UnixMilli(5000).In(time.UTC),
						Duration:  4,
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
			setupMock:      func(*MockBackupBackendService) {},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockBackends := &MockBackupBackendService{}
			cfg := model.NewConfig()
			_ = cfg.AddRoutine(&model.BackupRoutine{Name: "routine1"})

			tt.setupMock(mockBackends)

			svc := &Service{
				config:       cfg,
				backupReader: mockBackends,
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
	testScheduleBackupValidation(
		t,
		"/v1/backups/schedule/",
		func(svc *Service, w http.ResponseWriter, req *http.Request) {
			svc.ScheduleFullBackup(w, req)
		},
	)
}

func TestService_TriggerFullBackup(t *testing.T) {
	testScheduleBackupValidation(
		t,
		"/v1/backups/full/",
		func(svc *Service, w http.ResponseWriter, req *http.Request) {
			svc.TriggerFullBackup(w, req)
		},
	)
}

func TestService_ScheduleIncrementalBackup(t *testing.T) {
	testScheduleBackupValidation(
		t,
		"/v1/backups/schedule/incremental/",
		func(svc *Service, w http.ResponseWriter, req *http.Request) {
			svc.ScheduleIncrementalBackup(w, req)
		},
	)
}

func TestService_TriggerIncrementalBackup(t *testing.T) {
	testScheduleBackupValidation(
		t,
		"/v1/backups/incremental/",
		func(svc *Service, w http.ResponseWriter, req *http.Request) {
			svc.TriggerIncrementalBackup(w, req)
		},
	)
}

func testScheduleBackupValidation(
	t *testing.T,
	pathPrefix string,
	handler func(*Service, http.ResponseWriter, *http.Request),
) {
	t.Helper()

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

			req := httptest.NewRequest(http.MethodPost, pathPrefix+tt.routineName, nil)
			if tt.delayParam != "" {
				q := req.URL.Query()
				q.Add("delay", tt.delayParam)
				req.URL.RawQuery = q.Encode()
			}
			req.SetPathValue("name", tt.routineName)

			w := httptest.NewRecorder()
			handler(svc, w, req)

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
