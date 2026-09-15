package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/preflight"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// TestConfigEndpoints_ValidationCoverage is the executable form of one rule: a config
// change probes what it added or altered, and nothing else.
//
// It exists because the rule used to be a per-handler decision, and not a rule at all -
// updating a cluster was validated but adding one was not, and storage was never
// validated on any path. Every handler now runs the same check, so what this pins is the
// delta each endpoint produces: it drives a real preflight.Checker and records which
// storage was probed and which clusters were reached.
func TestConfigEndpoints_ValidationCoverage(t *testing.T) {
	tests := []struct {
		name           string
		call           func(*Service, http.ResponseWriter, *http.Request)
		method         string
		body           string
		pathValue      string
		expectStorage  []string
		expectClusters []string
		expectStatus   int
	}{
		{
			name:           "add cluster",
			call:           (*Service).AddAerospikeCluster,
			method:         http.MethodPost,
			pathValue:      "new-cluster",
			body:           marshalToString(clusterDTO(3000)),
			expectClusters: []string{"new-cluster"},
			expectStatus:   http.StatusCreated,
		},
		{
			name:           "update cluster",
			call:           (*Service).UpdateAerospikeCluster,
			method:         http.MethodPut,
			pathValue:      "cluster1",
			body:           marshalToString(clusterDTO(3001)),
			expectClusters: []string{"cluster1"},
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
			name:          "add storage",
			call:          (*Service).AddStorage,
			method:        http.MethodPost,
			pathValue:     "new-storage",
			body:          marshalToString(dto.Storage{LocalStorage: &dto.LocalStorage{Path: "/tmp/new"}}),
			expectStorage: []string{"/tmp/new"},
			expectStatus:  http.StatusCreated,
		},
		{
			name:          "update storage",
			call:          (*Service).UpdateStorage,
			method:        http.MethodPut,
			pathValue:     "storage1",
			body:          marshalToString(dto.Storage{LocalStorage: &dto.LocalStorage{Path: "/tmp/moved"}}),
			expectStorage: []string{"/tmp/moved"},
			expectStatus:  http.StatusOK,
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
			expectClusters: []string{"cluster1"},
			expectStorage:  []string{"/tmp/backup"},
			expectStatus:   http.StatusCreated,
		},
		{
			// Only the schedule differs from what routine1 already has, so the routine
			// still reads and writes exactly what it did.
			name:         "update routine, schedule only",
			call:         (*Service).UpdateRoutine,
			method:       http.MethodPut,
			pathValue:    "routine1",
			body:         marshalToString(validRoutineDTO()),
			expectStatus: http.StatusOK,
		},
		{
			name:      "update routine, namespaces changed",
			call:      (*Service).UpdateRoutine,
			method:    http.MethodPut,
			pathValue: "routine1",
			body: marshalToString(func() dto.BackupRoutine {
				routine := validRoutineDTO()
				routine.Namespaces = ptr.Of([]string{"source-ns1"})

				return routine
			}()),
			expectClusters: []string{"cluster1"},
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
			// Enabling changes nothing about what the routine reads or writes, so
			// there is nothing the check has not already seen.
			name:         "enable routine",
			call:         (*Service).EnableRoutine,
			method:       http.MethodPost,
			pathValue:    "routine1",
			expectStatus: http.StatusNoContent,
		},
		{
			name:         "disable routine",
			call:         (*Service).DisableRoutine,
			method:       http.MethodPost,
			pathValue:    "routine1",
			expectStatus: http.StatusNoContent,
		},
		{
			// A policy names neither a cluster nor a storage, and a routine that uses
			// an edited one still reads and writes the same places.
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

			// The handler probes on its own goroutine, so the delta arrives over a
			// channel rather than being read straight after the call.
			checked := make(chan *model.BackupConfig, 1)
			checker := preflight.NewMockChecker(ctrl)
			checker.EXPECT().
				Check(gomock.Any(), gomock.Any()).
				Do(func(_ context.Context, delta *model.BackupConfig) { checked <- delta }).
				Times(1)
			svc.checker = checker

			req := httptest.NewRequestWithContext(
				t.Context(), tt.method, "/v1/config/"+tt.pathValue, strings.NewReader(tt.body),
			)
			req.SetPathValue("name", tt.pathValue)
			w := httptest.NewRecorder()

			tt.call(svc, w, req)

			// A handler that rejected the request never reaches the check, so an empty
			// expectation would otherwise pass for the wrong reason.
			assert.Equal(t, tt.expectStatus, w.Code, w.Body.String())

			var delta *model.BackupConfig
			select {
			case delta = <-checked:
			case <-time.After(5 * time.Second):
				t.Fatal("the handler never handed a configuration to the checker")
			}

			assert.ElementsMatch(t, tt.expectStorage, storagePaths(delta), "storage to probe")
			assert.ElementsMatch(t, tt.expectClusters, clusterNames(delta), "clusters to reach")
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

	withUnused, err := cfg.ToModel()
	if err != nil {
		t.Fatalf("failed to build test config: %v", err)
	}
	svc.config.SetBackupConfig(withUnused.BackupConfigCopy())
}

func storagePaths(delta *model.BackupConfig) []string {
	var paths []string
	for _, s := range delta.Storage {
		paths = append(paths, s.GetPath())
	}

	return paths
}

func clusterNames(delta *model.BackupConfig) []string {
	var names []string
	for name := range delta.AerospikeClusters {
		names = append(names, name)
	}

	return names
}
