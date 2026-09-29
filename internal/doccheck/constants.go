package doccheck

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// StringConstants returns the values of every string constant of the named type
// declared in the given package directory.
//
// A set of allowed values is usually declared as typed constants and nowhere as a
// list, so this reads them from source: a value added next year is covered
// without anyone remembering to add it to a test.
func StringConstants(t *testing.T, root, pkgDir, typeName string) []string {
	t.Helper()

	fset := token.NewFileSet()
	dir := filepath.Join(root, pkgDir)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", pkgDir, err)
	}

	var values []string

	for _, entry := range entries {
		if !isSourceFile(entry.Name()) {
			continue
		}

		file, err := parser.ParseFile(fset, filepath.Join(dir, entry.Name()), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", entry.Name(), err)
		}

		values = append(values, stringConstantsInFile(file, typeName)...)
	}

	return values
}

func stringConstantsInFile(file *ast.File, typeName string) []string {
	var values []string

	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}

		for _, spec := range gen.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok || !isNamedType(valueSpec.Type, typeName) {
				continue
			}

			for _, value := range valueSpec.Values {
				literal, ok := value.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					continue
				}

				if unquoted, err := strconv.Unquote(literal.Value); err == nil {
					values = append(values, unquoted)
				}
			}
		}
	}

	return values
}

func isNamedType(expr ast.Expr, name string) bool {
	ident, ok := expr.(*ast.Ident)

	return ok && ident.Name == name
}
