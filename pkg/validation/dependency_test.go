package validation

import (
	"reflect"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/modules/schema"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

// This test ensures that the `validate` and `dto` packages are not dependent on schema.
func TestPackageDoesNotDependOnSchema(t *testing.T) {
	// resolves to "github.com/aerospike/aerospike-backup-service/v#/modules/schema"
	forbidden := reflect.TypeFor[schema.Schemas]().PkgPath()

	current := getCurrentPackage(t)
	seen := map[string]bool{}

	var check func(pkg *packages.Package)
	check = func(pkg *packages.Package) {
		if seen[pkg.ID] {
			return
		}
		seen[pkg.ID] = true

		require.NotEqual(t, pkg.ID, forbidden, "package %s depends on schema", pkg)

		for _, imp := range pkg.Imports {
			check(imp)
		}
	}

	check(current)
}

func getCurrentPackage(t *testing.T) *packages.Package {
	t.Helper()

	cfg := &packages.Config{
		Mode: packages.NeedImports | packages.NeedDeps,
	}

	pkgs, err := packages.Load(cfg, ".")
	require.NoError(t, err, "failed to load current package")
	require.NotEmpty(t, pkgs, "no packages found in current directory")

	return pkgs[0]
}
