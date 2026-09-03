package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestService_GetAllFullBackups(t *testing.T) {
	tests := []struct {
		name           string
		queryParams    map[string]string
		setupMock      func(*service.MockBackupReader)
		expectedStatus int
		expectedBody   map[string][]dto.BackupDetails
	}{
		{
			name: "successful retrieval",
			queryParams: map[string]string{
				"from": "1000",
				"to":   "2000",
			},
			setupMock: func(m *service.MockBackupReader) {
				m.EXPECT().GetBackups(gomock.Any(), gomock.Any()).
					Return([]model.BackupDetails{{Key: "backup1", BackupMetadata: model.BackupMetadata{
						Created:  time.UnixMilli(1000).In(time.UTC),
						Finished: time.UnixMilli(5000).In(time.UTC),
					}}}, nil)
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string][]dto.BackupDetails{
				"routine1": {
					{
						Key:               "backup1",
						Timestamp:         1000,
						Created:           time.UnixMilli(1000).In(time.UTC),
						Finished:          time.UnixMilli(5000).In(time.UTC),
						FinishedTimestamp: 5000,
						Duration:          4,
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
			setupMock:      func(*service.MockBackupReader) {},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			backupReader := service.NewMockBackupReader(ctrl)
			cfg := model.NewConfig()
			_ = cfg.AddRoutine(&model.BackupRoutine{Name: "routine1"})

			tt.setupMock(backupReader)

			svc := &Service{
				config:       cfg,
				backupReader: backupReader,
			}

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/backups/full", nil)
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

func TestService_TriggerIncrementalBackup(t *testing.T) {
	testScheduleBackupValidation(
		t,
		"/v1/backups/incremental/",
		func(svc *Service, w http.ResponseWriter, req *http.Request) {
			svc.TriggerIncrementalBackup(w, req)
		},
	)
}

func TestService_ScheduleBackupHappyPath(t *testing.T) {
	config := model.NewConfig()
	_ = config.AddRoutine(&model.BackupRoutine{Name: "test-routine"})

	svc := &Service{
		config: config,
	}

	const incrementalURL = "/v1/backups/incremental/test-routine?delay=1000"
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, incrementalURL, nil)
	req.SetPathValue("name", "test-routine")
	w := httptest.NewRecorder()

	var (
		calledRoutineName string
		calledDelay       time.Duration
	)
	svc.scheduleBackup(w, req, func(routine *model.BackupRoutine, delay time.Duration) error {
		calledRoutineName = routine.Name
		calledDelay = delay
		return nil
	})

	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Equal(t, "test-routine", calledRoutineName)
	assert.Equal(t, time.Second, calledDelay)
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
			config := model.NewConfig()
			_ = config.AddRoutine(&model.BackupRoutine{Name: tt.routineName})

			svc := &Service{
				backupScheduler: service.NewBackupScheduler(nil, nil),
				config:          config,
			}

			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, pathPrefix+tt.routineName, nil)
			if tt.delayParam != "" {
				q := req.URL.Query()
				q.Add("delay", tt.delayParam)
				req.URL.RawQuery = q.Encode()
			}
			req.SetPathValue("name", tt.routineName)

			w := httptest.NewRecorder()
			handler(svc, w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestService_CancelCurrentBackup(t *testing.T) {
	svc := &Service{}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/backups/cancel/", nil)
	w := httptest.NewRecorder()
	svc.CancelCurrentBackup(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestService_CancelCurrentBackup_RoutineNotFound(t *testing.T) {
	cfg := model.NewConfig()
	svc := &Service{config: cfg}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/backups/cancel/routine1", nil)
	req.SetPathValue("name", "routine1")
	w := httptest.NewRecorder()
	svc.CancelCurrentBackup(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestService_CancelCurrentBackup_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	cfg := model.NewConfig()
	_ = cfg.AddRoutine(&model.BackupRoutine{Name: "routine1"})

	mockRegistry := service.NewMockBackupStateRegistry(ctrl)
	mockRegistry.EXPECT().Cancel("routine1")

	svc := &Service{config: cfg, registry: mockRegistry}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/backups/cancel/routine1", nil)
	req.SetPathValue("name", "routine1")
	w := httptest.NewRecorder()
	svc.CancelCurrentBackup(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestService_GetCurrentBackupInfo(t *testing.T) {
	tests := []struct {
		name           string
		routineName    string
		setupSvc       func(*Service, *gomock.Controller)
		expectedStatus int
	}{
		{
			name:           "missing routine name",
			routineName:    "",
			setupSvc:       func(*Service, *gomock.Controller) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "routine not found",
			routineName:    "unknown",
			setupSvc:       func(*Service, *gomock.Controller) {},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:        "success",
			routineName: "routine1",
			setupSvc: func(svc *Service, ctrl *gomock.Controller) {
				_ = svc.config.AddRoutine(&model.BackupRoutine{Name: "routine1"})
				mockRegistry := service.NewMockBackupStateRegistry(ctrl)
				mockRegistry.EXPECT().GetRoutineState(gomock.Any()).Return(model.RoutineState{
					LastRunTime: model.NewNoBackupTime(),
					NextRunTime: model.NewNoBackupTime(),
				})
				svc.registry = mockRegistry
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			svc := &Service{config: model.NewConfig()}
			tt.setupSvc(svc, ctrl)

			req := httptest.NewRequestWithContext(
				t.Context(), http.MethodGet, "/v1/backups/currentBackup/"+tt.routineName, nil,
			)
			req.SetPathValue("name", tt.routineName)
			w := httptest.NewRecorder()

			svc.GetCurrentBackupInfo(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedStatus == http.StatusOK {
				var response dto.RoutineState
				require.NoError(t, json.NewDecoder(w.Body).Decode(&response))
			}
		})
	}
}

func TestService_GetFullBackupsForRoutine(t *testing.T) {
	tests := []struct {
		name           string
		routineName    string
		queryParams    map[string]string
		setupMock      func(*service.MockBackupReader)
		expectedStatus int
	}{
		{
			name:           "invalid time bounds",
			routineName:    "routine1",
			queryParams:    map[string]string{"from": "invalid"},
			setupMock:      func(*service.MockBackupReader) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing routine name",
			routineName:    "",
			setupMock:      func(*service.MockBackupReader) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "routine not found",
			routineName:    "unknown",
			setupMock:      func(*service.MockBackupReader) {},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:        "backend error",
			routineName: "routine1",
			setupMock: func(m *service.MockBackupReader) {
				m.EXPECT().GetBackups(gomock.Any(), gomock.Any()).
					Return([]model.BackupDetails(nil), assert.AnError)
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:        "success",
			routineName: "routine1",
			setupMock: func(m *service.MockBackupReader) {
				m.EXPECT().GetBackups(gomock.Any(), gomock.Any()).
					Return([]model.BackupDetails{{Key: "backup1"}}, nil)
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockBackends := service.NewMockBackupReader(ctrl)
			tt.setupMock(mockBackends)

			cfg := model.NewConfig()
			_ = cfg.AddRoutine(&model.BackupRoutine{Name: "routine1"})

			svc := &Service{config: cfg, backupReader: mockBackends}

			req := httptest.NewRequestWithContext(
				t.Context(), http.MethodGet, "/v1/backups/full/"+tt.routineName, nil,
			)
			req.SetPathValue("name", tt.routineName)
			q := req.URL.Query()
			for k, v := range tt.queryParams {
				q.Add(k, v)
			}
			req.URL.RawQuery = q.Encode()
			w := httptest.NewRecorder()

			svc.GetFullBackupsForRoutine(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedStatus == http.StatusOK {
				var response []dto.BackupDetails
				require.NoError(t, json.NewDecoder(w.Body).Decode(&response))
				assert.Len(t, response, 1)
			}
		})
	}
}

func TestService_GetAllIncrementalBackups(t *testing.T) {
	tests := []struct {
		name           string
		queryParams    map[string]string
		setupMock      func(*service.MockBackupReader)
		expectedStatus int
	}{
		{
			name:           "invalid time bounds",
			queryParams:    map[string]string{"from": "invalid"},
			setupMock:      func(*service.MockBackupReader) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "success",
			setupMock: func(m *service.MockBackupReader) {
				m.EXPECT().GetBackups(gomock.Any(), gomock.Any()).
					Return([]model.BackupDetails{{Key: "incr1"}}, nil)
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockBackends := service.NewMockBackupReader(ctrl)
			tt.setupMock(mockBackends)

			cfg := model.NewConfig()
			_ = cfg.AddRoutine(&model.BackupRoutine{Name: "routine1"})

			svc := &Service{config: cfg, backupReader: mockBackends}

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/backups/incremental", nil)
			q := req.URL.Query()
			for k, v := range tt.queryParams {
				q.Add(k, v)
			}
			req.URL.RawQuery = q.Encode()
			w := httptest.NewRecorder()

			svc.GetAllIncrementalBackups(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedStatus == http.StatusOK {
				var response map[string][]dto.BackupDetails
				require.NoError(t, json.NewDecoder(w.Body).Decode(&response))
				assert.Contains(t, response, "routine1")
			}
		})
	}
}

func TestService_GetIncrementalBackupsForRoutine(t *testing.T) {
	tests := []struct {
		name           string
		routineName    string
		setupMock      func(*service.MockBackupReader)
		expectedStatus int
	}{
		{
			name:           "missing routine name",
			routineName:    "",
			setupMock:      func(*service.MockBackupReader) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "routine not found",
			routineName:    "unknown",
			setupMock:      func(*service.MockBackupReader) {},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:        "success",
			routineName: "routine1",
			setupMock: func(m *service.MockBackupReader) {
				m.EXPECT().GetBackups(gomock.Any(), gomock.Any()).
					Return([]model.BackupDetails{{Key: "incr1"}}, nil)
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockBackends := service.NewMockBackupReader(ctrl)
			tt.setupMock(mockBackends)

			cfg := model.NewConfig()
			_ = cfg.AddRoutine(&model.BackupRoutine{Name: "routine1"})

			svc := &Service{config: cfg, backupReader: mockBackends}

			req := httptest.NewRequestWithContext(
				t.Context(), http.MethodGet, "/v1/backups/incremental/"+tt.routineName, nil,
			)
			req.SetPathValue("name", tt.routineName)
			w := httptest.NewRecorder()

			svc.GetIncrementalBackupsForRoutine(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedStatus == http.StatusOK {
				var response []dto.BackupDetails
				require.NoError(t, json.NewDecoder(w.Body).Decode(&response))
				assert.Len(t, response, 1)
			}
		})
	}
}
