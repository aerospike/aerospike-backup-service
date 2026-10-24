package doccheck_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/internal/doccheck"
	"github.com/aerospike/aerospike-backup-service/v3/internal/server"
	"github.com/stretchr/testify/require"
)

const (
	// apiPath and sysPath are the defaults the server is started with. Prose
	// quotes the default deployment, so that is what the docs are matched against.
	apiPath = "/v1"
	sysPath = "/"
)

// docFiles are the documents under check, listed rather than discovered so that
// a run here matches a run in CI: the repository picks up untracked scratch
// documents that only exist on one machine, and scanning the directory would
// silently check those too.
//
//nolint:gochecknoglobals // a table, read by the tests in this package.
var docFiles = []string{
	"docs/api-examples.md",
	"docs/architecture.md",
	"docs/configuration.md",
	"docs/installation.md",
	"docs/monitoring.md",
	"docs/security.md",
	"docs/migration.md",
	"README.md",
}

// TestOpenAPIMatchesRoutes is the one hop in the documentation chain that
// generation cannot close.
//
// Everything a reader sees comes from docs/openapi.json: the published API
// browser renders it, clients generate from it, and every endpoint named in the
// documents is rendered from it by an <!-- Endpoint --> marker. But the document
// itself is built by swag from annotations on the handlers, and those are a
// second, independent statement of what the service serves — the mux is the
// first. A route registered without an annotation, or an annotation left behind
// by a route that moved, shows up here as a one-line difference.
func TestOpenAPIMatchesRoutes(t *testing.T) {
	registered := registeredRoutes()
	documented := openAPIOperations(t)

	for key := range registered {
		t.Run("registered/"+key, func(t *testing.T) {
			require.Truef(t, documented[key],
				"%s is registered in internal/server/router.go but has no operation in "+
					"docs/openapi.json; add the swagger annotation to the handler and run \"make docs\"",
				key)
		})
	}

	for key := range documented {
		t.Run("documented/"+key, func(t *testing.T) {
			require.Truef(t, registered[key],
				"docs/openapi.json describes %s, which internal/server/router.go does not register",
				key)
		})
	}
}

// openAPIOperations reads the generated contract as "METHOD /path" keys.
func openAPIOperations(t *testing.T) map[string]bool {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(doccheck.Root(t), "docs", "openapi.json"))
	require.NoError(t, err, "read docs/openapi.json")

	var document struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}

	require.NoError(t, json.Unmarshal(content, &document), "parse docs/openapi.json")

	operations := make(map[string]bool)

	for path, methods := range document.Paths {
		for method := range methods {
			operations[strings.ToUpper(method)+" "+path] = true
		}
	}

	return operations
}

func registeredRoutes() map[string]bool {
	registered := make(map[string]bool)
	for _, route := range server.Routes(apiPath, sysPath, nil) {
		registered[route.Method+" "+route.Pattern] = true
	}

	return registered
}
