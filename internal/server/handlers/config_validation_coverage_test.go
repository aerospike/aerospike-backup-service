package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/preflight"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// TestConfigEndpoints_ValidationCoverage is the executable form of one rule: a config
// change that points the service at a cluster or storage runs the advisory preflight
// check, and a change that can only remove or stop work does not.
//
// It exists because the previous rule was not a rule at all - updating a cluster was
// validated but adding one was not, and storage was never validated on any path. A
// handler added later that silently stops validating fails here.
func TestConfigEndpoints_ValidationCoverage(t *testing.T) {
	tests := []struct {
		name           string
		call           func(*Service, http.ResponseWriter, *http.Request)
		method         string
		body           string
		pathValue      string
		expectValidate bool
		expectStatus   int
	}{
		{
			name:           "add cluster",
			call:           (*Service).AddAerospikeCluster,
			method:         http.MethodPost,
			pathValue:      "new-cluster",
			body:           marshalToString(clusterDTO(3000)),
			expectValidate: true,
			expectStatus:   http.StatusCreated,
		},
		{
			name:           "update cluster",
			call:           (*Service).UpdateAerospikeCluster,
			method:         http.MethodPut,
			pathValue:      "cluster1",
			body:           marshalToString(clusterDTO(3001)),
			expectValidate: true,
			expectStatus:   http.StatusOK,
		},
		{
			name:         "delete cluster",
			call:         (*Service).DeleteAerospikeCluster,
			method:       http.MethodDelete,
			pathValue:    "unused-cluster",
			expectStatus: http.StatusNoContent,
		},
		{
			name:           "add storage",
			call:           (*Service).AddStorage,
			method:         http.MethodPost,
			pathValue:      "new-storage",
			body:           marshalToString(dto.Storage{LocalStorage: &dto.LocalStorage{Path: "/tmp/new"}}),
			expectValidate: true,
			expectStatus:   http.StatusCreated,
		},
		{
			name:           "update storage",
			call:           (*Service).UpdateStorage,
			method:         http.MethodPut,
			pathValue:      "storage1",
			body:           marshalToString(dto.Storage{LocalStorage: &dto.LocalStorage{Path: "/tmp/moved"}}),
			expectValidate: true,
			expectStatus:   http.StatusOK,
		},
		{
			name:         "delete storage",
			call:         (*Service).DeleteStorage,
			method:       http.MethodDelete,
			pathValue:    "unused-storage",
			expectStatus: http.StatusNoContent,
		},
		{
			name:           "add routine",
			call:           (*Service).AddRoutine,
			method:         http.MethodPost,
			pathValue:      "new-routine",
			body:           marshalToString(validRoutineDTO()),
			expectValidate: true,
			expectStatus:   http.StatusCreated,
		},
		{
			name:           "update routine",
			call:           (*Service).UpdateRoutine,
			method:         http.MethodPut,
			pathValue:      "routine1",
			body:           marshalToString(validRoutineDTO()),
			expectValidate: true,
			expectStatus:   http.StatusOK,
		},
		{
			name:         "delete routine",
			call:         (*Service).DeleteRoutine,
			method:       http.MethodDelete,
			pathValue:    "routine1",
			expectStatus: http.StatusNoContent,
		},
		{
			// Enabling a routine points the service at a cluster and a storage again.
			name:           "enable routine",
			call:           (*Service).EnableRoutine,
			method:         http.MethodPost,
			pathValue:      "routine1",
			expectValidate: true,
			expectStatus:   http.StatusNoContent,
		},
		{
			name:         "disable routine",
			call:         (*Service).DisableRoutine,
			method:       http.MethodPost,
			pathValue:    "routine1",
			expectStatus: http.StatusNoContent,
		},
		{
			// A policy names neither a cluster nor a storage.
			name:         "add policy",
			call:         (*Service).AddPolicy,
			method:       http.MethodPost,
			pathValue:    "new-policy",
			body:         marshalToString(dto.BackupPolicy{Parallel: ptr.Of(8)}),
			expectStatus: http.StatusCreated,
		},
		{
			name:         "update policy",
			call:         (*Service).UpdatePolicy,
			method:       http.MethodPut,
			pathValue:    "test-policy",
			body:         "{}",
			expectStatus: http.StatusOK,
		},
		{
			name:         "delete policy",
			call:         (*Service).DeletePolicy,
			method:       http.MethodDelete,
			pathValue:    "unused-policy",
			expectStatus: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			svc := setupTestService(t)
			addValidBackupConfig(svc)
			addUnusedEntities(t, svc)

			// DisableRoutine cancels any running job for the routine it just disabled.
			registry := service.NewMockBackupStateRegistry(ctrl)
			registry.EXPECT().Cancel(gomock.Any()).AnyTimes()
			svc.registry = registry

			checker := preflight.NewMockChecker(ctrl)
			svc.checker = checker
			if tt.expectValidate {
				checker.EXPECT().Check(gomock.Any(), gomock.Eq(svc.config)).Times(1)
			}

			req := httptest.NewRequestWithContext(
				t.Context(), tt.method, "/v1/config/"+tt.pathValue, strings.NewReader(tt.body),
			)
			req.SetPathValue("name", tt.pathValue)
			w := httptest.NewRecorder()

			tt.call(svc, w, req)

			// A handler that rejected the request never reaches the check, so the
			// expectation above would pass for the wrong reason.
			assert.Equal(t, tt.expectStatus, w.Code, w.Body.String())
		})
	}
}

func clusterDTO(port int) dto.AerospikeCluster {
	return dto.AerospikeCluster{
		SeedNodes: []dto.SeedNode{{HostName: "localhost", Port: dto.Port(port)}},
	}
}

func validRoutineDTO() dto.BackupRoutine {
	return dto.BackupRoutine{
		SourceCluster: "cluster1",
		Storage:       "storage1",
		BackupPolicy:  "test-policy",
		IntervalCron:  "@daily",
		Namespaces:    ptr.Of([]string{}),
	}
}

// addUnusedEntities adds entities no routine references, so the delete cases have
// something they are allowed to remove.
func addUnusedEntities(t *testing.T, svc *Service) {
	t.Helper()

	cfg := dto.NewConfigFromModel(svc.config)
	cfg.AerospikeClusters["unused-cluster"] = &dto.AerospikeCluster{
		SeedNodes: []dto.SeedNode{{HostName: "localhost", Port: 3000}},
	}
	cfg.Storage["unused-storage"] = &dto.Storage{LocalStorage: &dto.LocalStorage{Path: "/tmp/unused"}}
	cfg.BackupPolicies["unused-policy"] = &dto.BackupPolicy{Parallel: ptr.Of(1)}

	model, err := cfg.ToModel()
	if err != nil {
		t.Fatalf("failed to build test config: %v", err)
	}
	svc.config.SetBackupConfig(model.BackupConfigCopy())
}
