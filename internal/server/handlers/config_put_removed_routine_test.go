package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const twoRoutineConfig = `{
  "aerospike-clusters": {"c1": {"seed-nodes": [{"host-name": "localhost", "port": 3000}]}},
  "storage": {"s1": {"local-storage": {"path": "/tmp/backups"}}},
  "backup-policies": {"p1": {}},
  "backup-routines": {
    "keep": {
      "backup-policy": "p1", "source-cluster": "c1", "storage": "s1", "interval-cron": "@daily", "namespaces": ["test"]
    },
    "remove": {
      "backup-policy": "p1", "source-cluster": "c1", "storage": "s1", "interval-cron": "@daily", "namespaces": ["test"]
    }
  }
}`

// A routine removed through PUT /v1/config loses its periodic jobs and its pending on-demand
// backups; the routine it keeps loses neither.
func TestService_UpdateConfig_RemovedRoutineIsUnscheduled(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := newServiceWithNamespaceValidator(t)

	scheduler, err := service.NewScheduler(slog.Default()) // not started: jobs stay pending
	require.NoError(t, err)
	registry := service.NewMockBackupStateRegistry(ctrl)
	registry.EXPECT().RequestHistorySync(gomock.Any()).AnyTimes()
	backupScheduler := service.NewBackupScheduler(scheduler, service.NewBackupOrchestrator(nil, nil, nil, nil, nil))
	svc.configApplier = service.NewConfigApplier(backupScheduler, registry, svc.config)

	put := func(body string) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/v1/config", strings.NewReader(body))
		w := httptest.NewRecorder()
		svc.UpdateConfig(w, req)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	}
	jobs := func() []string {
		keys, err := scheduler.GetJobKeys()
		require.NoError(t, err)
		names := make([]string, 0, len(keys))
		for _, key := range keys {
			names = append(names, key.Name())
		}
		return names
	}

	put(twoRoutineConfig)
	for _, name := range []string{"keep", "remove"} {
		routine, err := svc.config.Routine(name)
		require.NoError(t, err)
		require.NoError(t, backupScheduler.TriggerAdHocFullBackup(routine, time.Hour))
	}
	require.ElementsMatch(t,
		[]string{"keep-full", "remove-full", "keep-adhoc-full-1", "remove-adhoc-full-2"}, jobs(),
		"precondition: both routines scheduled")

	// GET-edit-PUT: drop the "remove" routine.
	body, err := decoder.Marshal(dto.NewConfigFromModel(svc.config), decoder.JSON, true)
	require.NoError(t, err)
	var cfg map[string]any
	require.NoError(t, json.Unmarshal(body, &cfg))
	delete(cfg["backup-routines"].(map[string]any), "remove")
	edited, err := json.Marshal(cfg)
	require.NoError(t, err)
	put(string(edited))

	assert.ElementsMatch(t, []string{"keep-full", "keep-adhoc-full-1"}, jobs())
}
