// Package doccheck holds the checks that keep the hand-written documentation
// honest.
//
// The generated documentation is reproduced byte-for-byte from the code, so it
// cannot drift. Three things outside it can, and the tests here check each one
// against the code, so a doc that no longer matches fails the build rather than
// a support ticket:
//   - a default written down in a `default:` struct tag and again in the code
//     that applies it;
//   - the routes docs/openapi.json describes and the ones the router registers;
//   - a field name written in prose, against the properties docs/openapi.json
//     declares.
package doccheck

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// Root returns the repository root. Tests run with the package directory as the
// working directory, so every path in this package is resolved through here.
func Root(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("%s is not the repository root: %v", root, err)
	}

	return root
}

// DefaultTag is a documented default: the `default:` struct tag on a DTO field.
// It is what the DTO tables, the OpenAPI document and the JSON schema publish.
type DefaultTag struct {
	// Type is the DTO type name, e.g. "BackupPolicy".
	Type string
	// Field is the Go field name, e.g. "SocketTimeout".
	Field string
	// Key is the serialized name, e.g. "socket-timeout".
	Key string
	// Value is the published default, verbatim from the tag.
	Value string
	// Pos is "file:line" of the field.
	Pos string
}

// ID names the tag the way the tests refer to it.
func (d DefaultTag) ID() string { return d.Type + "." + d.Field }

// DefaultTags returns every `default:` struct tag declared in the given package
// directory, so a test can assert that each one is accounted for. Reading the
// tags from source rather than from a hand-kept list is the point: a field added
// next year is covered without anyone remembering to add it.
func DefaultTags(t *testing.T, root, pkgDir string) []DefaultTag {
	t.Helper()

	fset := token.NewFileSet()
	dir := filepath.Join(root, pkgDir)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", pkgDir, err)
	}

	var tags []DefaultTag

	for _, entry := range entries {
		if !isSourceFile(entry.Name()) {
			continue
		}

		file, err := parser.ParseFile(fset, filepath.Join(dir, entry.Name()), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", entry.Name(), err)
		}

		tags = append(tags, defaultTagsInFile(fset, root, file)...)
	}

	return tags
}

func isSourceFile(name string) bool {
	return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
}

func defaultTagsInFile(fset *token.FileSet, root string, file *ast.File) []DefaultTag {
	var tags []DefaultTag

	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}

		structType, ok := spec.Type.(*ast.StructType)
		if !ok {
			return true
		}

		for _, field := range structType.Fields.List {
			tags = append(tags, defaultTagOf(fset, root, spec.Name.Name, field)...)
		}

		return true
	})

	return tags
}

func defaultTagOf(fset *token.FileSet, root, typeName string, field *ast.Field) []DefaultTag {
	if field.Tag == nil || len(field.Names) == 0 {
		return nil
	}

	tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))

	value, ok := tag.Lookup("default")
	if !ok {
		return nil
	}

	position := fset.Position(field.Pos())
	relative, err := filepath.Rel(root, position.Filename)
	if err != nil {
		relative = position.Filename
	}

	return []DefaultTag{{
		Type:  typeName,
		Field: field.Names[0].Name,
		Key:   serializedKey(tag),
		Value: value,
		Pos:   relative + ":" + strconv.Itoa(position.Line),
	}}
}

// serializedKey returns the name the field carries on the wire.
func serializedKey(tag reflect.StructTag) string {
	for _, key := range []string{"yaml", "json"} {
		if value, ok := tag.Lookup(key); ok {
			if name, _, _ := strings.Cut(value, ","); name != "" {
				return name
			}
		}
	}

	return ""
}
