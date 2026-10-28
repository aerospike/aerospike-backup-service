package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/internal/server/configuration"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const fileBackedConfig = `
service:
  http:
    port: 8099
  logger:
    level: DEBUG
aerospike-clusters:
  c1:
    seed-nodes: [{host-name: localhost, port: 3000}]
`

// newFileBackedService loads the configuration from a real file, the way the service does at
// startup, and wires the handlers to the file configuration manager that came with it.
func newFileBackedService(t *testing.T, applyCalls int) (*Service, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "aerospike-backup-service.yml")
	require.NoError(t, os.WriteFile(path, []byte(fileBackedConfig), 0o600))

	ctrl := gomock.NewController(t)
	prober := newMockTLSProber(ctrl)

	config, manager, err := configuration.Load(t.Context(), path, false, nil, prober)
	require.NoError(t, err)

	applier := service.NewMockConfigApplier(ctrl)
	applier.EXPECT().ApplyNewConfig(gomock.Any()).Return(nil).Times(applyCalls)

	return NewService(config, applier, nil, nil, nil, nil, nil, manager, prober), path
}

func addPolicyRequest(t *testing.T, name string) *http.Request {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/config/policies/"+name,
		strings.NewReader(marshalToString(dto.BackupPolicy{Parallel: new(2)})))
	req.SetPathValue("name", name)

	return req
}

// A change is written to the configuration file together with the service section the file
// already had, and the file reads back as the configuration the API now reports.
func TestChangeBackupConfig_PersistsToConfigFile(t *testing.T) {
	svc, path := newFileBackedService(t, 1)

	w := httptest.NewRecorder()
	svc.AddPolicy(w, addPolicyRequest(t, "p1"))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	reloaded, _, err := configuration.Load(t.Context(), path, false, nil,
		newMockTLSProber(gomock.NewController(t)))
	require.NoError(t, err)

	assert.Contains(t, reloaded.BackupConfigCopy().BackupPolicies, "p1")
	assert.Contains(t, reloaded.BackupConfigCopy().AerospikeClusters, "c1")
	assert.Equal(t, dto.NewConfigFromModel(svc.config), dto.NewConfigFromModel(reloaded))
}

// When the configuration file cannot be written, as with a read-only mount, the change is
// reported as not persisted and leaves the live configuration as it was; a retry of the same
// request fails the same way instead of reporting a conflict.
func TestChangeBackupConfig_UnwritableConfigFileChangesNothing(t *testing.T) {
	svc, path := newFileBackedService(t, 0)

	// A directory in place of the file fails the write for every user, root included, where
	// a permission bit would not.
	require.NoError(t, os.Remove(path))
	require.NoError(t, os.Mkdir(path, 0o700))

	for range 2 {
		w := httptest.NewRecorder()
		svc.AddPolicy(w, addPolicyRequest(t, "p1"))
		require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), "failed to write configuration")
	}

	w := httptest.NewRecorder()
	svc.ReadConfig(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/config", nil))
	assert.NotContains(t, w.Body.String(), `"p1"`)
}
