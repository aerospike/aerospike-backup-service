package archcheck_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/internal/doccheck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const servicePackage = "github.com/aerospike/aerospike-backup-service/v3/pkg/service"

// backupModes are the directories under pkg/service that hold a backup mode each. A package in
// one of them, at any depth, belongs to that mode. Every other package under pkg/service is
// shared by the modes.
var backupModes = []string{"scanbackup", "serverbackup"}

// TestBackupModesAreSeparate enforces the split between the backup modes: a package of a mode is
// imported only from that mode. So the shared packages use a mode only through interfaces, and
// the two modes do not import each other. Only the composition root, which is outside
// pkg/service, picks a mode.
func TestBackupModesAreSeparate(t *testing.T) {
	t.Parallel()

	packages := servicePackages(t)
	for _, mode := range backupModes {
		require.Contains(t, packages, mode, "the backup mode has no package; did it move?")
	}

	for _, pkg := range packages {
		for _, imported := range serviceImports(t, pkg) {
			mode := modeOf(imported)
			if mode == "" {
				continue
			}

			assert.Equalf(t, mode, modeOf(pkg), "pkg/service/%s imports pkg/service/%s", pkg, imported)
		}
	}
}

// modeOf returns the backup mode the package pkg of pkg/service belongs to, and "" for a shared
// package.
func modeOf(pkg string) string {
	top, _, _ := strings.Cut(pkg, "/")
	if slices.Contains(backupModes, top) {
		return top
	}

	return ""
}

// servicePackages returns the packages under pkg/service, each relative to it. pkg/service
// itself is ".".
func servicePackages(t *testing.T) []string {
	t.Helper()

	root := filepath.Join(doccheck.Root(t), "pkg", "service")

	var packages []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".go" {
			return err
		}

		pkg, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}

		if pkg = filepath.ToSlash(pkg); !slices.Contains(packages, pkg) {
			packages = append(packages, pkg)
		}

		return nil
	})
	require.NoError(t, err)

	return packages
}

// serviceImports returns the packages under pkg/service that the package pkg of pkg/service
// imports, tests included, each relative to pkg/service.
func serviceImports(t *testing.T, pkg string) []string {
	t.Helper()

	files, err := filepath.Glob(filepath.Join(doccheck.Root(t), "pkg", "service", pkg, "*.go"))
	require.NoError(t, err)

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
