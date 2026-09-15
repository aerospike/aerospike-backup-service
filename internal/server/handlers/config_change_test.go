package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Secrets are merged before the static-field comparison, so a GET-edit-PUT round trip
// carrying a redacted service.https.key-file-password is accepted.
func TestUpdateConfig_AcceptsRedactedKeyFilePassword(t *testing.T) {
	svc := setupTestService(t)
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
	svc := setupTestService(t)
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
