package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/internal/server/configuration"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// Secrets are merged before the static-field comparison, so a GET-edit-PUT round trip
// carrying a redacted service.https.key-file-password is accepted.
func TestUpdateConfig_AcceptsRedactedKeyFilePassword(t *testing.T) {
	svc := newServiceWithNamespaceValidator(t)
	svc.config.ServiceConfig.ServerHTTPS = &model.ServerConfigHTTPS{
		ListenerConfig:  model.ListenerConfig{Disabled: true},
		CertFile:        "/c.pem",
		KeyFile:         "/k.pem",
		KeyFilePassword: "real-passphrase",
	}
	addValidBackupConfig(svc)

	// Simulate GET /v1/config: the response body is the redacted DTO.
	body, err := decoder.Marshal(dto.NewConfigFromModel(svc.config), decoder.JSON, true)
	require.NoError(t, err)
	require.Contains(t, string(body), `"key-file-password":"[secret]"`)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/v1/config", strings.NewReader(string(body)))
	w := httptest.NewRecorder()
	svc.UpdateConfig(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

// Updating a cluster or a policy invalidates every routine that uses it, so the
// scheduled jobs (which hold a routine snapshot) are rebuilt with the new definition.
func TestUpdateClusterAndPolicy_InvalidateDependentRoutines(t *testing.T) {
	svc := newServiceWithNamespaceValidator(t)
	entities := addValidBackupConfig(svc)
	svc.config.PopInvalidatedRoutineNames() // drain the AddRoutine invalidation

	clusterReq := httptest.NewRequestWithContext(t.Context(), http.MethodPut,
		"/v1/config/clusters/"+entities.clusterName,
		strings.NewReader(marshalToString(dto.AerospikeCluster{
			SeedNodes: []dto.SeedNode{{HostName: "new-host", Port: 3000}},
		})))
	clusterReq.SetPathValue("name", entities.clusterName)
	w := httptest.NewRecorder()
	svc.UpdateAerospikeCluster(w, clusterReq)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, []string{entities.routineName}, svc.config.PopInvalidatedRoutineNames())

	policyReq := httptest.NewRequestWithContext(t.Context(), http.MethodPut,
		"/v1/config/policies/"+entities.policyName,
		strings.NewReader(marshalToString(dto.BackupPolicy{Parallel: ptr.Of(2)})))
	policyReq.SetPathValue("name", entities.policyName)
	w = httptest.NewRecorder()
	svc.UpdatePolicy(w, policyReq)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, []string{entities.routineName}, svc.config.PopInvalidatedRoutineNames())
}

// A configuration change becomes visible only once it is persisted: when Write fails, the
// in-memory config stays as it was, the scheduler is not touched, and the client gets a server
// error rather than a 400.
func TestChangeBackupConfig_WriteFailureLeavesConfigUntouched(t *testing.T) {
	ctrl := gomock.NewController(t)
	manager := configuration.NewMockManager(ctrl)
	manager.EXPECT().Write(gomock.Any(), gomock.Any()).Return(errors.New("disk full"))
	applier := service.NewMockConfigApplier(ctrl)
	applier.EXPECT().ApplyNewConfig().Times(0)
	nsValidator := aerospike.NewMockNamespaceValidator(ctrl)
	nsValidator.EXPECT().Validate(gomock.Any(), gomock.Any()).AnyTimes()

	svc := NewService(model.NewConfig(), applier, nil, nil, nil, nil, nil, manager, nsValidator, newMockTLSProber(ctrl))

	body := marshalToString(dto.AerospikeCluster{SeedNodes: []dto.SeedNode{{HostName: "localhost", Port: 3000}}})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/config/clusters/c1", strings.NewReader(body))
	req.SetPathValue("name", "c1")
	w := httptest.NewRecorder()
	svc.AddAerospikeCluster(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
	assert.NotContains(t, svc.config.BackupConfigCopy().AerospikeClusters, "c1",
		"a change that was not persisted must not stay in memory")
}

// A failed write changes nothing: not the configuration, and not the set of routines waiting
// to be rescheduled and rescanned.
func TestChangeBackupConfig_WriteFailureLeavesNoPendingInvalidation(t *testing.T) {
	ctrl := gomock.NewController(t)
	manager := configuration.NewMockManager(ctrl)
	manager.EXPECT().Write(gomock.Any(), gomock.Any()).Return(errors.New("disk full"))
	applier := service.NewMockConfigApplier(ctrl)
	applier.EXPECT().ApplyNewConfig().Times(0)

	svc := newServiceWithNamespaceValidator(t)
	svc.configurationManager = manager
	svc.configApplier = applier
	entities := addValidBackupConfig(svc)
	svc.config.PopInvalidatedRoutineNames() // drain what building the fixture invalidated

	body := marshalToString(dto.AerospikeCluster{
		SeedNodes:    []dto.SeedNode{{HostName: "localhost", Port: 3000}},
		ClusterLabel: "updated",
	})
	req := httptest.NewRequestWithContext(
		t.Context(), http.MethodPut, "/v1/config/clusters/"+entities.clusterName, strings.NewReader(body))
	req.SetPathValue("name", entities.clusterName)
	w := httptest.NewRecorder()
	svc.UpdateAerospikeCluster(w, req)

	require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
	assert.Empty(t, svc.config.PopInvalidatedRoutineNames(),
		"a change that was not persisted must not schedule a rescan")
	assert.Empty(t, svc.config.BackupConfigCopy().AerospikeClusters[entities.clusterName].ClusterLabel)
}

// The candidate configuration is what reaches the configuration file, and the live one is
// replaced only afterwards.
func TestChangeBackupConfig_WritesCandidateBeforePublishing(t *testing.T) {
	ctrl := gomock.NewController(t)
	manager := configuration.NewMockManager(ctrl)
	manager.EXPECT().Write(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, written *model.Config) error {
			assert.Contains(t, written.BackupConfigCopy().AerospikeClusters, "c1",
				"the persisted configuration must already carry the change")

			return nil
		})
	applier := service.NewMockConfigApplier(ctrl)
	applier.EXPECT().ApplyNewConfig().Return(nil)
	nsValidator := aerospike.NewMockNamespaceValidator(ctrl)
	nsValidator.EXPECT().Validate(gomock.Any(), gomock.Any()).AnyTimes()

	svc := NewService(model.NewConfig(), applier, nil, nil, nil, nil, nil, manager, nsValidator, newMockTLSProber(ctrl))

	body := marshalToString(dto.AerospikeCluster{SeedNodes: []dto.SeedNode{{HostName: "localhost", Port: 3000}}})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/config/clusters/c1", strings.NewReader(body))
	req.SetPathValue("name", "c1")
	w := httptest.NewRecorder()
	svc.AddAerospikeCluster(w, req)

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Contains(t, svc.config.BackupConfigCopy().AerospikeClusters, "c1")
}

// A rejected change is still a bad request: only the persist step reports a server error.
func TestChangeBackupConfig_RejectedChangeIsBadRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	manager := configuration.NewMockManager(ctrl)
	manager.EXPECT().Write(gomock.Any(), gomock.Any()).Times(0)
	applier := service.NewMockConfigApplier(ctrl)
	applier.EXPECT().ApplyNewConfig().Times(0)
	nsValidator := aerospike.NewMockNamespaceValidator(ctrl)
	nsValidator.EXPECT().Validate(gomock.Any(), gomock.Any()).AnyTimes()

	svc := NewService(model.NewConfig(), applier, nil, nil, nil, nil, nil, manager, nsValidator, newMockTLSProber(ctrl))
	require.NoError(t, svc.config.AddCluster("c1", &model.AerospikeCluster{
		SeedNodes: []model.SeedNode{{HostName: "localhost", Port: 3000}},
	}))

	body := marshalToString(dto.AerospikeCluster{SeedNodes: []dto.SeedNode{{HostName: "localhost", Port: 3000}}})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/config/clusters/c1", strings.NewReader(body))
	req.SetPathValue("name", "c1")
	w := httptest.NewRecorder()
	svc.AddAerospikeCluster(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}
