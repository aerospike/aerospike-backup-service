package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var validRestoreCluster = dto.AerospikeCluster{SeedNodes: []dto.SeedNode{{HostName: "localhost", Port: 3000}}}
var validRestoreStorage = dto.Storage{LocalStorage: &dto.LocalStorage{Path: "/tmp/backup"}}

func TestService_RestoreHandlers(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		handler func(*Service, http.ResponseWriter, *http.Request)
		jobID   model.RestoreJobID
	}{
		{
			name:    "full",
			path:    "/v1/restore/full",
			handler: (*Service).RestoreFullHandler,
			jobID:   1,
		},
		{
			name:    "incremental",
			path:    "/v1/restore/incremental",
			handler: (*Service).RestoreIncrementalHandler,
			jobID:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			mockManager := service.NewMockRestoreManager(ctrl)
			mockManager.EXPECT().Restore(gomock.Any(), gomock.Any()).Return(tt.jobID, nil)

			svc := &Service{
				sysCtx:         t.Context(),
				config:         model.NewConfig(),
				restoreManager: mockManager,
				tlsProber:      newMockTLSProber(ctrl),
			}

			body := marshalToString(dto.RestoreRequest{
				DestinationClusterConfig: dto.DestinationClusterConfig{Cluster: &validRestoreCluster},
				StorageConfig:            dto.StorageConfig{Storage: &validRestoreStorage},
				Policy:                   &dto.RestorePolicy{},
				BackupDataPath:           "test/path",
			})

			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, tt.path, strings.NewReader(body))
			w := httptest.NewRecorder()

			tt.handler(svc, w, req)

			assert.Equal(t, http.StatusAccepted, w.Code)
			assert.Equal(t, fmt.Sprintf("%d", tt.jobID), w.Body.String())
		})
	}
}

func TestService_restoreByPath(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		setupMock      func(*service.MockRestoreManager)
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "invalid json",
			requestBody:    "{noField : 1}",
			setupMock:      func(*service.MockRestoreManager) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid JSON payload",
		},
		{
			name:           "validation error - missing backup data path",
			requestBody:    marshalToString(dto.RestoreRequest{}),
			setupMock:      func(*service.MockRestoreManager) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "to model error - unknown cluster name",
			requestBody: marshalToString(dto.RestoreRequest{
				DestinationClusterConfig: dto.DestinationClusterConfig{Name: "does-not-exist"},
				StorageConfig:            dto.StorageConfig{Storage: &validRestoreStorage},
				Policy:                   &dto.RestorePolicy{},
				BackupDataPath:           "test/path",
			}),
			setupMock:      func(*service.MockRestoreManager) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "restore manager error",
			requestBody: marshalToString(dto.RestoreRequest{
				DestinationClusterConfig: dto.DestinationClusterConfig{Cluster: &validRestoreCluster},
				StorageConfig:            dto.StorageConfig{Storage: &validRestoreStorage},
				Policy:                   &dto.RestorePolicy{},
				BackupDataPath:           "test/path",
			}),
			setupMock: func(m *service.MockRestoreManager) {
				m.EXPECT().Restore(gomock.Any(), gomock.Any()).Return(model.RestoreJobID(0), errors.New("boom"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "boom",
		},
		{
			name: "success",
			requestBody: marshalToString(dto.RestoreRequest{
				DestinationClusterConfig: dto.DestinationClusterConfig{Cluster: &validRestoreCluster},
				StorageConfig:            dto.StorageConfig{Storage: &validRestoreStorage},
				Policy:                   &dto.RestorePolicy{},
				BackupDataPath:           "test/path",
			}),
			setupMock: func(m *service.MockRestoreManager) {
				m.EXPECT().Restore(gomock.Any(), gomock.Any()).Return(model.RestoreJobID(42), nil)
			},
			expectedStatus: http.StatusAccepted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			mockManager := service.NewMockRestoreManager(ctrl)
			tt.setupMock(mockManager)

			svc := &Service{
				sysCtx:         t.Context(),
				config:         model.NewConfig(),
				restoreManager: mockManager,
				tlsProber:      newMockTLSProber(ctrl),
			}

			req := httptest.NewRequestWithContext(
				t.Context(), http.MethodPost, "/v1/restore/full", strings.NewReader(tt.requestBody),
			)
			w := httptest.NewRecorder()

			svc.restoreByPath(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}

func TestService_RestoreByTimeHandler(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		setupMock      func(*service.MockRestoreManager)
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "invalid json",
			requestBody:    "{noField : 1}",
			setupMock:      func(*service.MockRestoreManager) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid JSON payload",
		},
		{
			name: "validation error - missing time",
			requestBody: marshalToString(dto.RestoreTimestampRequest{
				Routine: "daily",
				Policy:  &dto.TimestampRestorePolicy{},
			}),
			setupMock:      func(*service.MockRestoreManager) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "to model error - unknown routine",
			requestBody: marshalToString(dto.RestoreTimestampRequest{
				DestinationClusterConfig: dto.DestinationClusterConfig{Cluster: &validRestoreCluster},
				Policy:                   &dto.TimestampRestorePolicy{},
				Time:                     1739538000000,
				Routine:                  "does-not-exist",
			}),
			setupMock:      func(*service.MockRestoreManager) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "restore manager error",
			requestBody: marshalToString(dto.RestoreTimestampRequest{
				DestinationClusterConfig: dto.DestinationClusterConfig{Cluster: &validRestoreCluster},
				StorageConfig:            dto.StorageConfig{Storage: &validRestoreStorage},
				Policy:                   &dto.TimestampRestorePolicy{},
				Time:                     1739538000000,
				Routine:                  "daily",
			}),
			setupMock: func(m *service.MockRestoreManager) {
				m.EXPECT().RestoreByTime(gomock.Any(), gomock.Any()).Return(model.RestoreJobID(0), errors.New("boom"))
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "boom",
		},
		{
			name: "success",
			requestBody: marshalToString(dto.RestoreTimestampRequest{
				DestinationClusterConfig: dto.DestinationClusterConfig{Cluster: &validRestoreCluster},
				StorageConfig:            dto.StorageConfig{Storage: &validRestoreStorage},
				Policy:                   &dto.TimestampRestorePolicy{},
				Time:                     1739538000000,
				Routine:                  "daily",
			}),
			setupMock: func(m *service.MockRestoreManager) {
				m.EXPECT().RestoreByTime(gomock.Any(), gomock.Any()).Return(model.RestoreJobID(7), nil)
			},
			expectedStatus: http.StatusAccepted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			mockManager := service.NewMockRestoreManager(ctrl)
			tt.setupMock(mockManager)

			svc := &Service{
				sysCtx:         t.Context(),
				config:         model.NewConfig(),
				restoreManager: mockManager,
				tlsProber:      newMockTLSProber(ctrl),
			}

			req := httptest.NewRequestWithContext(
				t.Context(), http.MethodPost, "/v1/restore/timestamp", strings.NewReader(tt.requestBody),
			)
			w := httptest.NewRecorder()

			svc.RestoreByTimeHandler(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}

func TestService_RestoreStatusHandler(t *testing.T) {
	tests := []struct {
		name           string
		jobIDParam     string
		setupMock      func(*service.MockRestoreManager)
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "empty job id",
			jobIDParam:     "",
			setupMock:      func(*service.MockRestoreManager) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid job id",
			jobIDParam:     "not-a-number",
			setupMock:      func(*service.MockRestoreManager) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:       "job not found",
			jobIDParam: "5",
			setupMock: func(m *service.MockRestoreManager) {
				m.EXPECT().JobStatus(model.RestoreJobID(5)).Return(nil, service.NewJobNotFoundError(5))
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:       "other error",
			jobIDParam: "5",
			setupMock: func(m *service.MockRestoreManager) {
				m.EXPECT().JobStatus(model.RestoreJobID(5)).Return(nil, errors.New("boom"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "boom",
		},
		{
			name:       "success",
			jobIDParam: "5",
			setupMock: func(m *service.MockRestoreManager) {
				status := &model.RestoreJobStatus{
					Status:   model.RestoreSuccess,
					Counters: models.NewRestoreStats(),
				}
				m.EXPECT().JobStatus(model.RestoreJobID(5)).Return(status, nil)
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			mockManager := service.NewMockRestoreManager(ctrl)
			tt.setupMock(mockManager)

			svc := &Service{restoreManager: mockManager}

			req := httptest.NewRequestWithContext(
				t.Context(), http.MethodGet, "/v1/restore/status/"+tt.jobIDParam, nil,
			)
			req.SetPathValue("jobId", tt.jobIDParam)
			w := httptest.NewRecorder()

			svc.RestoreStatusHandler(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
			if tt.expectedStatus == http.StatusOK {
				var response dto.RestoreJobStatus
				require.NoError(t, json.NewDecoder(w.Body).Decode(&response))
			}
		})
	}
}

func TestService_RetrieveRestoreJobs(t *testing.T) {
	tests := []struct {
		name           string
		queryParams    map[string]string
		setupMock      func(*service.MockRestoreManager)
		expectedStatus int
	}{
		{
			name: "invalid time bounds",
			queryParams: map[string]string{
				"from": "invalid",
			},
			setupMock:      func(*service.MockRestoreManager) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "invalid status filter",
			queryParams: map[string]string{
				"status": "unknown-status",
			},
			setupMock:      func(*service.MockRestoreManager) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "successful retrieval",
			queryParams: map[string]string{
				"from":   "1000",
				"to":     "2000",
				"status": "success",
			},
			setupMock: func(m *service.MockRestoreManager) {
				m.EXPECT().GetFilteredJobs(gomock.Any(), gomock.Any()).Return(map[model.RestoreJobID]*model.RestoreJobStatus{
					1: {Status: model.RestoreSuccess, Counters: models.NewRestoreStats()},
				})
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			mockManager := service.NewMockRestoreManager(ctrl)
			tt.setupMock(mockManager)

			svc := &Service{restoreManager: mockManager}

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/restore/jobs", nil)
			q := req.URL.Query()
			for k, v := range tt.queryParams {
				q.Add(k, v)
			}
			req.URL.RawQuery = q.Encode()
			w := httptest.NewRecorder()

			svc.RetrieveRestoreJobs(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedStatus == http.StatusOK {
				var response map[string]dto.RestoreJobStatus
				require.NoError(t, json.NewDecoder(w.Body).Decode(&response))
				assert.Contains(t, response, "1")
			}
		})
	}
}

func TestService_RetrieveConfig(t *testing.T) {
	tests := []struct {
		name           string
		routineName    string
		timestamp      string
		setupSvc       func(*Service)
		setupMock      func(*service.MockConfigRetriever)
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "missing routine name",
			routineName:    "",
			timestamp:      "1000",
			setupSvc:       func(*Service) {},
			setupMock:      func(*service.MockConfigRetriever) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  errMissingRoutineName.Error(),
		},
		{
			name:           "missing timestamp",
			routineName:    "routine1",
			timestamp:      "",
			setupSvc:       func(*Service) {},
			setupMock:      func(*service.MockConfigRetriever) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid timestamp",
			routineName:    "routine1",
			timestamp:      "not-a-number",
			setupSvc:       func(*Service) {},
			setupMock:      func(*service.MockConfigRetriever) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "routine not found",
			routineName:    "unknown-routine",
			timestamp:      "1000",
			setupSvc:       func(*Service) {},
			setupMock:      func(*service.MockConfigRetriever) {},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:        "retriever error",
			routineName: "routine1",
			timestamp:   "1000",
			setupSvc: func(s *Service) {
				_ = s.config.AddRoutine(&model.BackupRoutine{Name: "routine1"})
			},
			setupMock: func(m *service.MockConfigRetriever) {
				m.EXPECT().
					RetrieveConfiguration(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("boom"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "boom",
		},
		{
			name:        "success",
			routineName: "routine1",
			timestamp:   "1000",
			setupSvc: func(s *Service) {
				_ = s.config.AddRoutine(&model.BackupRoutine{Name: "routine1"})
			},
			setupMock: func(m *service.MockConfigRetriever) {
				m.EXPECT().
					RetrieveConfiguration(gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]byte("zip-content"), nil)
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			mockRetriever := service.NewMockConfigRetriever(ctrl)
			tt.setupMock(mockRetriever)

			svc := &Service{
				config:          model.NewConfig(),
				configRetriever: mockRetriever,
			}
			tt.setupSvc(svc)

			req := httptest.NewRequestWithContext(
				t.Context(), http.MethodGet, "/v1/retrieve/configuration/"+tt.routineName+"/"+tt.timestamp, nil,
			)
			req.SetPathValue("name", tt.routineName)
			req.SetPathValue("timestamp", tt.timestamp)
			w := httptest.NewRecorder()

			svc.RetrieveConfig(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
			if tt.expectedStatus == http.StatusOK {
				assert.Equal(t, "zip-content", w.Body.String())
				assert.Equal(t, "application/zip", w.Header().Get("Content-Type"))
			}
		})
	}
}

func TestService_CancelRestoreHandler(t *testing.T) {
	tests := []struct {
		name           string
		jobIDParam     string
		setupMock      func(*service.MockRestoreManager)
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "invalid job id",
			jobIDParam:     "not-a-number",
			setupMock:      func(*service.MockRestoreManager) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:       "job not found",
			jobIDParam: "5",
			setupMock: func(m *service.MockRestoreManager) {
				m.EXPECT().CancelRestore(model.RestoreJobID(5)).Return(service.NewJobNotFoundError(5))
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:       "other error",
			jobIDParam: "5",
			setupMock: func(m *service.MockRestoreManager) {
				m.EXPECT().CancelRestore(model.RestoreJobID(5)).Return(errors.New("boom"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "failed to cancel restore",
		},
		{
			name:       "success",
			jobIDParam: "5",
			setupMock: func(m *service.MockRestoreManager) {
				m.EXPECT().CancelRestore(model.RestoreJobID(5)).Return(nil)
			},
			expectedStatus: http.StatusAccepted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			mockManager := service.NewMockRestoreManager(ctrl)
			tt.setupMock(mockManager)

			svc := &Service{restoreManager: mockManager}

			req := httptest.NewRequestWithContext(
				t.Context(), http.MethodPost, "/v1/restore/cancel/"+tt.jobIDParam, nil,
			)
			req.SetPathValue("jobId", tt.jobIDParam)
			w := httptest.NewRecorder()

			svc.CancelRestoreHandler(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}
