//go:build integration

package integration

import (
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/internal/app"
	"github.com/aerospike/aerospike-backup-service/v3/internal/server"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcAerospike "github.com/testcontainers/testcontainers-go/modules/aerospike"
)

const (
	routineName = "integrationRoutine"
	namespace   = "test"
	setName     = "filteredSet"
)

type env struct {
	backupDir string
	server    *httptest.Server
	asHost    string
	asPort    int
}

type envOptions struct {
	filterExpression string
	setList          string
}

func setupEnv(t *testing.T, opts ...envOptions) *env {
	t.Helper()

	var options envOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	ctx := t.Context()

	ctr, err := tcAerospike.Run(ctx, "aerospike/aerospike-server:8.1",
		testcontainers.WithEnv(map[string]string{
			"REPL_FACTOR": "1",
		}),
	)
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, ctr)

	host, err := ctr.Host(ctx)
	require.NoError(t, err)

	port, err := ctr.MappedPort(ctx, "3000/tcp")
	require.NoError(t, err)

	configDir := t.TempDir()
	backupDir := t.TempDir()
	cfgPath := filepath.Join(configDir, "config.yml")
	err = os.WriteFile(cfgPath, []byte(configYAML(host, int(port.Num()), backupDir, options)), 0o600)
	require.NoError(t, err)

	scheduler, svc, err := app.InitComponents(ctx, cfgPath, false)
	require.NoError(t, err)

	scheduler.Start(ctx)
	t.Cleanup(func() { scheduler.Stop() })

	srv := httptest.NewServer(server.NewServeMux("/v1", "/", svc))
	t.Cleanup(srv.Close)

	return &env{
		backupDir: backupDir,
		server:    srv,
		asHost:    host,
		asPort:    int(port.Num()),
	}
}

func configYAML(host string, port int, backupDir string, opts envOptions) string {
	filterExpLine := ""
	if opts.filterExpression != "" {
		filterExpLine = fmt.Sprintf("    filter-exp: %q\n", opts.filterExpression)
	}

	setListLine := ""
	if opts.setList != "" {
		setListLine = fmt.Sprintf("    set-list:\n      - %s\n", opts.setList)
	}

	return fmt.Sprintf(`
service:
  logger:
    level: ERROR
aerospike-clusters:
  testCluster:
    seed-nodes:
      - host-name: %s
        port: %d
    use-services-alternate: true
storage:
  local:
    local-storage:
      path: %s
backup-policies:
  defaultPolicy:
    parallel: 1
    retention:
      full: 10
      incremental: 0
backup-routines:
  %s:
    backup-policy: defaultPolicy
    source-cluster: testCluster
    storage: local
    interval-cron: '@yearly'
    namespaces:
      - %s
%s%s`, host, port, backupDir, routineName, namespace, setListLine, filterExpLine)
}
