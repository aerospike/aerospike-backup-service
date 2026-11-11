package archcheck_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path"
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

// TestBackupModesAreSeparate enforces the split between the backup modes inside pkg/service:
// there, a package of a mode is imported only from that mode. So the shared packages use a mode
// only through interfaces, and the two modes do not import each other. Packages outside
// pkg/service are not checked.
func TestBackupModesAreSeparate(t *testing.T) {
	t.Parallel()

	root := filepath.Join(doccheck.Root(t), "pkg", "service")
	seenModes := make(map[string]bool)

	err := filepath.WalkDir(root, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(file) != ".go" {
			return nil
		}

		dir, err := filepath.Rel(root, filepath.Dir(file))
		if err != nil {
			return err
		}

		pkg := filepath.ToSlash(dir)
		seenModes[modeOf(pkg)] = true

		for _, imported := range serviceImports(t, file) {
			if mode := modeOf(imported); mode != "" {
				assert.Equalf(t, mode, modeOf(pkg), "%s imports %s",
					path.Join("pkg/service", pkg), path.Join("pkg/service", imported))
			}
		}

		return nil
	})
	require.NoError(t, err)

	for _, mode := range backupModes {
		require.Truef(t, seenModes[mode], "backup mode %s has no package; did it move?", mode)
	}
}

func TestModeOf(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"scanbackup":                 "scanbackup",
		"scanbackup/restoreexecutor": "scanbackup",
		"serverbackup":               "serverbackup",
		".":                          "",
		"storage":                    "",
	}

	for pkg, want := range tests {
		assert.Equal(t, want, modeOf(pkg), pkg)
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

// serviceImports returns the packages under pkg/service that the Go file imports, each relative
// to pkg/service.
func serviceImports(t *testing.T, file string) []string {
	t.Helper()

	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
	require.NoError(t, err)

	var imports []string
	for _, spec := range parsed.Imports {
		imported, err := strconv.Unquote(spec.Path.Value)
		require.NoError(t, err)

		if sub, ok := strings.CutPrefix(imported, servicePackage+"/"); ok {
			imports = append(imports, sub)
		}
	}

	return imports
}
