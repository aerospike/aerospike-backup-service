package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/require"
)

// A policy that reads its encryption key from a Secret Agent can be added before any routine
// uses it, as when an operator defines policies first and routines later; it is the routine
// that must name the agent the key is resolved with.
func TestAddPolicyThenRoutine_KeySecretAgent(t *testing.T) {
	svc := newServiceWithNamespaceValidator(t)
	require.NoError(t, svc.config.AddSecretAgent("sa", &model.SecretAgent{
		ConnectionType: "tcp", Address: "secret-agent.aerospike.svc", Port: ptr.Of(model.Port(3005)),
	}))
	require.NoError(t, svc.config.AddCluster("c1", &model.AerospikeCluster{
		SeedNodes: []model.SeedNode{{HostName: "localhost", Port: 3000}},
	}))
	require.NoError(t, svc.config.AddStorage("s1", &model.LocalStorage{Path: "/tmp/backups"}))

	post := func(path, name, body string, handler http.HandlerFunc) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path+name, strings.NewReader(body))
		req.SetPathValue("name", name)
		w := httptest.NewRecorder()
		handler(w, req)

		return w
	}
	routine := func(agent string) string {
		return `{"backup-policy":"enc-policy","source-cluster":"c1","storage":"s1",` +
			`"interval-cron":"@daily","namespaces":["test"]` + agent + `}`
	}

	w := post("/v1/config/policies/", "enc-policy",
		`{"encryption":{"mode":"AES256","key-secret":"secrets:abs:enckey"}}`, svc.AddPolicy)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	w = post("/v1/config/routines/", "with-agent", routine(`,"secret-agent":"sa"`), svc.AddRoutine)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	w = post("/v1/config/routines/", "without-agent", routine(""), svc.AddRoutine)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "backup routine 'without-agent' validation error: secret-agent")
}
