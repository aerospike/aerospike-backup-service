package archcheck_test

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/internal/doccheck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const servicePackage = "github.com/aerospike/aerospike-backup-service/v3/pkg/service"

// The packages of each backup mode, relative to pkg/service.
var (
	scanPackages   = []string{"scanbackup", "backupexecutor", "restoreexecutor"}
	serverPackages = []string{"serverbackup"}
)

// TestBackupModesAreSeparate enforces the split between the backup modes. pkg/service holds the
// code both modes share and works with a mode only through interfaces, so it imports no mode
// package. The two modes do not import each other. Only the composition root picks a mode.
func TestBackupModesAreSeparate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		packages  []string
		forbidden []string
	}{
		{name: "shared code imports no mode", packages: []string{""}, forbidden: append(scanPackages, serverPackages...)},
		{name: "scan mode does not import server mode", packages: scanPackages, forbidden: serverPackages},
		{name: "server mode does not import scan mode", packages: serverPackages, forbidden: scanPackages},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			for _, pkg := range tt.packages {
				for _, imported := range serviceImports(t, pkg) {
					assert.NotContainsf(t, tt.forbidden, imported, "pkg/service/%s imports pkg/service/%s", pkg, imported)
				}
			}
		})
	}
}

// serviceImports returns the packages under pkg/service that the package pkg of pkg/service
// imports, tests included, each relative to pkg/service.
func serviceImports(t *testing.T, pkg string) []string {
	t.Helper()

	dir := filepath.Join(doccheck.Root(t), "pkg", "service", pkg)
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	require.NoError(t, err)
	require.NotEmpty(t, files, "no Go files in %s; did the package move?", dir)

	var imports []string
	for _, file := range files {
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		require.NoError(t, err)

		for _, spec := range parsed.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			require.NoError(t, err)

			if sub, ok := strings.CutPrefix(path, servicePackage+"/"); ok {
				imports = append(imports, sub)
			}
		}
	}

	return imports
}
