package decoder

import (
	"go/types"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

// TestSecretFieldsDoNotShareAStructWithUnexportedFields guards the assumption redactValue's
// struct case relies on: once any field in a struct needs redacting, every unexported field
// in that same struct is read-only to reflect and gets zeroed wholesale rather than preserved
// (see BKRS-416) - so a struct carrying a secret must not also carry unrelated private state.
//
// It walks every named struct type declared in pkg/model and pkg/dto and fails on one that
// has both a redact.Redactable field and an unexported field, so a future struct shaped that
// way is caught here rather than by silently losing data in production logs.
func TestSecretFieldsDoNotShareAStructWithUnexportedFields(t *testing.T) {
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedTypes | packages.NeedSyntax}
	pkgs, err := packages.Load(cfg,
		"github.com/aerospike/aerospike-backup-service/v3/pkg/redact",
		"github.com/aerospike/aerospike-backup-service/v3/pkg/model/...",
		"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/...",
	)
	require.NoError(t, err)
	require.Zero(t, packages.PrintErrors(pkgs), "package(s) failed to load cleanly")

	redactable := redactableInterface(t, pkgs)

	for _, pkg := range pkgs {
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			obj, ok := scope.Lookup(name).(*types.TypeName)
			if !ok {
				continue
			}

			structType, ok := obj.Type().Underlying().(*types.Struct)
			if !ok {
				continue
			}

			var secretFields, privateFields []string
			for field := range structType.Fields() {
				if isRedactableFieldType(field.Type(), redactable) {
					secretFields = append(secretFields, field.Name())
				}
				if !field.Exported() {
					privateFields = append(privateFields, field.Name())
				}
			}

			if len(secretFields) > 0 && len(privateFields) > 0 {
				t.Errorf("%s.%s has secret field(s) %v alongside unexported field(s) %v: "+
					"the redaction walk zeroes unexported fields wholesale once any field needs "+
					"redacting, silently dropping their state",
					pkg.PkgPath, obj.Name(), secretFields, privateFields)
			}
		}
	}
}

// isRedactableFieldType mirrors isRedactable in masking.go: a Redactable value must have
// string as its underlying type, since redaction replaces it with its DisplayString.
func isRedactableFieldType(t types.Type, redactable *types.Interface) bool {
	basic, ok := t.Underlying().(*types.Basic)

	return ok && basic.Info()&types.IsString != 0 && types.Implements(t, redactable)
}

func redactableInterface(t *testing.T, pkgs []*packages.Package) *types.Interface {
	t.Helper()

	for _, pkg := range pkgs {
		if pkg.PkgPath != "github.com/aerospike/aerospike-backup-service/v3/pkg/redact" {
			continue
		}

		obj := pkg.Types.Scope().Lookup("Redactable")
		require.NotNil(t, obj, "pkg/redact.Redactable not found - has it been renamed?")

		iface, ok := obj.Type().Underlying().(*types.Interface)
		require.True(t, ok, "pkg/redact.Redactable is no longer an interface")

		return iface
	}

	t.Fatal("pkg/redact package not loaded")

	return nil
}
