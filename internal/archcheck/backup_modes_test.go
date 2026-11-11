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

const (
	modulePath = "github.com/aerospike/aerospike-backup-service/v3"
	serviceDir = "pkg/service"
	// compositionRoot is the one package that picks a backup mode.
	compositionRoot = "internal/app"
)

// backupModes are the directories under pkg/service that hold a backup mode each. A package in
// one of them, at any depth, belongs to that mode.
var backupModes = []string{"scanbackup", "serverbackup"}

// sourceDirs are the directories of the module that hold its Go code.
var sourceDirs = []string{"cmd", "internal", "pkg", "test"}

// TestBackupModesAreSeparate enforces the split between the backup modes: a package of a mode is
// imported only from that mode and from the composition root. So the rest of the service uses a
// mode only through interfaces, and the two modes do not import each other.
func TestBackupModesAreSeparate(t *testing.T) {
	t.Parallel()

	root := doccheck.Root(t)
	seenModes := make(map[string]bool)

	for _, sourceDir := range sourceDirs {
		err := filepath.WalkDir(filepath.Join(root, sourceDir), func(file string, entry fs.DirEntry, err error) error {
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

			for _, imported := range moduleImports(t, file) {
				mode := modeOf(imported)
				if mode == "" || pkg == compositionRoot {
					continue
				}

				assert.Equalf(t, mode, modeOf(pkg), "%s imports %s", pkg, imported)
			}

			return nil
		})
		require.NoError(t, err)
	}

	for _, mode := range backupModes {
		require.Truef(t, seenModes[mode], "backup mode %s has no package; did it move?", mode)
	}
}

func TestModeOf(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"pkg/service/scanbackup":                 "scanbackup",
		"pkg/service/scanbackup/restoreexecutor": "scanbackup",
		"pkg/service/serverbackup":               "serverbackup",
		"pkg/service":                            "",
		"pkg/service/storage":                    "",
		"internal/app":                           "",
		"pkg/scanbackup":                         "",
	}

	for pkg, want := range tests {
		assert.Equal(t, want, modeOf(pkg), pkg)
	}
}

// modeOf returns the backup mode the package belongs to, and "" for a package of no mode. pkg is
// relative to the module root.
func modeOf(pkg string) string {
	inService, ok := strings.CutPrefix(pkg, serviceDir+"/")
	if !ok {
		return ""
	}

	top, _, _ := strings.Cut(inService, "/")
	if slices.Contains(backupModes, top) {
		return top
	}

	return ""
}

// moduleImports returns the packages of this module that the Go file imports, each relative to
// the module root.
func moduleImports(t *testing.T, file string) []string {
	t.Helper()

	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
	require.NoError(t, err)

	var imports []string
	for _, spec := range parsed.Imports {
		imported, err := strconv.Unquote(spec.Path.Value)
		require.NoError(t, err)

		if pkg, ok := strings.CutPrefix(imported, modulePath+"/"); ok {
			imports = append(imports, path.Clean(pkg))
		}
	}

	return imports
}
