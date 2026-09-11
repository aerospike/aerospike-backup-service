// Package doccheck holds the checks that keep the hand-written documentation
// honest.
//
// The generated documentation is reproduced byte-for-byte from the code, so it
// cannot drift. Everything else can: prose that names an endpoint, a YAML
// snippet a reader will paste, a default written down in a struct tag and again
// in the code that applies it. The tests here read those artifacts and put them
// through the same parsers the service uses, so a doc that no longer matches the
// code fails the build rather than a support ticket.
package doccheck

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
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

// Snippet is one fenced code block from a Markdown file.
type Snippet struct {
	// File is the path relative to the repository root.
	File string
	// Line is the 1-based line of the block's opening fence.
	Line int
	// Language is the fence's info string, e.g. "yaml".
	Language string
	// Body is the block's content, without the fences.
	Body string
	// Generated reports whether build/docs rendered this block from source.
	Generated bool
}

// fence matches a fenced block and captures its language and body.
var fence = regexp.MustCompile("(?ms)^```([a-zA-Z0-9_-]*)[ \t]*\r?\n(.*?)^```[ \t]*$")

// marker matches the HTML comment that build/docs writes immediately above a
// block it renders, such as <!-- DefaultConfig --> or <!-- dto.Config -->.
var marker = regexp.MustCompile(`^<!--\s*[\w.]+\s*-->$`)

// isGenerated reports whether the fence starting at offset is preceded by a
// generator marker. A generated block is reproduced byte-for-byte from source
// and verified by "make docs-check", so checking its content again would only
// test the generator twice.
func isGenerated(text string, offset int) bool {
	preceding := strings.TrimRight(text[:offset], " \t\r\n")

	lastBreak := strings.LastIndex(preceding, "\n")

	return marker.MatchString(strings.TrimSpace(preceding[lastBreak+1:]))
}

// Snippets returns every fenced block in the given Markdown files whose language
// is one of langs. Paths are relative to the repository root.
func Snippets(t *testing.T, root string, files []string, langs ...string) []Snippet {
	t.Helper()

	wanted := make(map[string]bool, len(langs))
	for _, lang := range langs {
		wanted[lang] = true
	}

	var snippets []Snippet

	for _, file := range files {
		content, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}

		text := string(content)
		for _, match := range fence.FindAllStringSubmatchIndex(text, -1) {
			language := text[match[2]:match[3]]
			if !wanted[language] {
				continue
			}

			snippets = append(snippets, Snippet{
				File:      file,
				Line:      1 + strings.Count(text[:match[0]], "\n"),
				Language:  language,
				Body:      text[match[4]:match[5]],
				Generated: isGenerated(text, match[0]),
			})
		}
	}

	return snippets
}

// At renders the snippet's origin the way an editor understands it.
func (s Snippet) At() string {
	return s.File + ":" + strconv.Itoa(s.Line)
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
