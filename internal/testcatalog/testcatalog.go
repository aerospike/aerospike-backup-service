// Package testcatalog reads the integration tests in test/integration from source:
// which suite each Test* method belongs to, the doc comment that says what it
// checks, and the cases it runs. The docs generator renders the catalog into
// test/integration/README.md, and internal/doccheck holds the directory to the
// rules the catalog makes checkable.
package testcatalog

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// SuitesFile is the one file allowed to declare suites and their runners; every
// other *_test.go file declares only Test* methods.
const SuitesFile = "suite_test.go"

// Catalog is every test in one directory.
type Catalog struct {
	// Suites are in the order their runners are declared in SuitesFile.
	Suites []Suite
	// Orphans are Test* methods on a type no runner starts: they never run.
	Orphans []Test
	// Strays are declarations outside SuitesFile that are not Test* methods.
	Strays []Decl
	// Misplaced are Test* functions and methods declared in a non-test file, where
	// they escape the README and the checks here.
	Misplaced []Decl
}

// Suite is one testify suite and the Test* methods declared on it.
type Suite struct {
	// Type is the suite struct, e.g. "BackupSuite".
	Type string
	// Runner is the top-level test that runs it, e.g. "TestBackup".
	Runner string
	// Doc is the doc comment on Type.
	Doc string
	// Tests are ordered by file, then by position in the file.
	Tests []Test
}

// Test is one Test* method.
type Test struct {
	Suite string
	Name  string
	// File is the base name of the file declaring it.
	File string
	// Doc is the method's doc comment, empty when it has none.
	Doc string
	// Cases are the literal names of the s.Run subtests in its body, in order.
	Cases []string
}

// Summary is the first paragraph of the doc comment on one line, with a leading
// "TestName " dropped so it reads as a description of what the test does.
func (t Test) Summary() string {
	paragraph, _, _ := strings.Cut(strings.TrimSpace(t.Doc), "\n\n")
	summary := strings.Join(strings.Fields(paragraph), " ")

	rest, found := strings.CutPrefix(summary, t.Name+" ")
	if !found || rest == "" {
		return summary
	}

	return strings.ToUpper(rest[:1]) + rest[1:]
}

// Decl is a declaration found where only tests belong.
type Decl struct {
	// Pos is "file:line".
	Pos  string
	Name string
}

// Read parses every .go file in dir: the tests from *_test.go files, and from the
// others only any Test* declaration that should not be there. Build constraints are
// ignored, so the integration-tagged files are read like any other.
func Read(dir string) (Catalog, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return Catalog{}, err
	}

	fset := token.NewFileSet()
	r := reader{fset: fset, runners: map[string]string{}, suiteDocs: map[string]string{}}

	slices.Sort(files)

	for _, path := range files {
		file, parseErr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if parseErr != nil {
			return Catalog{}, fmt.Errorf("parse %s: %w", path, parseErr)
		}

		name := filepath.Base(path)
		if !strings.HasSuffix(name, "_test.go") {
			r.nonTestFile(file)
			continue
		}

		r.file(name, file)
	}

	return r.catalog(), nil
}

type reader struct {
	fset *token.FileSet
	// suiteOrder is the suite types in the order their runners appear.
	suiteOrder []string
	// runners maps a suite type to the Test function that runs it.
	runners   map[string]string
	suiteDocs map[string]string
	tests     []Test
	strays    []Decl
	misplaced []Decl
}

// nonTestFile records the Test* declarations in a file go test does not treat as
// a test file.
func (r *reader) nonTestFile(file *ast.File) {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || !strings.HasPrefix(fn.Name.Name, "Test") {
			continue
		}

		name := fn.Name.Name
		if fn.Recv != nil {
			name = receiverType(fn.Recv) + "." + name
		}

		r.misplaced = append(r.misplaced, r.decl(fn.Pos(), name))
	}
}

func (r *reader) file(name string, file *ast.File) {
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			r.funcDecl(name, decl)
		case *ast.GenDecl:
			r.genDecl(name, decl)
		}
	}
}

func (r *reader) funcDecl(file string, decl *ast.FuncDecl) {
	name := decl.Name.Name
	isTest := strings.HasPrefix(name, "Test")

	if decl.Recv == nil {
		if file == SuitesFile && isTest {
			r.runner(name, decl)
			return
		}

		r.stray(decl.Pos(), name)

		return
	}

	suiteType := receiverType(decl.Recv)
	if file == SuitesFile || !isTest {
		r.stray(decl.Pos(), suiteType+"."+name)
		return
	}

	r.tests = append(r.tests, Test{
		Suite: suiteType,
		Name:  name,
		File:  file,
		Doc:   decl.Doc.Text(),
		Cases: subtestNames(decl.Body),
	})
}

func (r *reader) genDecl(file string, decl *ast.GenDecl) {
	if decl.Tok == token.IMPORT {
		return
	}

	for _, spec := range decl.Specs {
		typeSpec, isType := spec.(*ast.TypeSpec)
		if file == SuitesFile && isType {
			doc := typeSpec.Doc
			if doc == nil {
				doc = decl.Doc
			}

			r.suiteDocs[typeSpec.Name.Name] = doc.Text()

			continue
		}

		r.stray(spec.Pos(), specName(spec))
	}
}

// runner records the suite a top-level test starts with suite.Run(t, new(X)).
func (r *reader) runner(name string, decl *ast.FuncDecl) {
	ast.Inspect(decl.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || !isSelector(call.Fun, "suite", "Run") || len(call.Args) != 2 {
			return true
		}

		if suiteType := newType(call.Args[1]); suiteType != "" {
			r.suiteOrder = append(r.suiteOrder, suiteType)
			r.runners[suiteType] = name
		}

		return false
	})
}

func (r *reader) stray(pos token.Pos, name string) {
	r.strays = append(r.strays, r.decl(pos, name))
}

func (r *reader) decl(pos token.Pos, name string) Decl {
	position := r.fset.Position(pos)

	return Decl{
		Pos:  fmt.Sprintf("%s:%d", filepath.Base(position.Filename), position.Line),
		Name: name,
	}
}

func (r *reader) catalog() Catalog {
	bySuite := map[string][]Test{}

	var orphans []Test

	for _, test := range r.tests {
		if _, runs := r.runners[test.Suite]; !runs {
			orphans = append(orphans, test)
			continue
		}

		bySuite[test.Suite] = append(bySuite[test.Suite], test)
	}

	suites := make([]Suite, 0, len(r.suiteOrder))
	for _, suiteType := range r.suiteOrder {
		suites = append(suites, Suite{
			Type:   suiteType,
			Runner: r.runners[suiteType],
			Doc:    r.suiteDocs[suiteType],
			Tests:  bySuite[suiteType],
		})
	}

	return Catalog{Suites: suites, Orphans: orphans, Strays: r.strays, Misplaced: r.misplaced}
}

// subtestNames returns the literal first argument of every s.Run(…) call in body.
func subtestNames(body *ast.BlockStmt) []string {
	var names []string

	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || !isSelector(call.Fun, "s", "Run") || len(call.Args) == 0 {
			return true
		}

		if lit, isLit := call.Args[0].(*ast.BasicLit); isLit && lit.Kind == token.STRING {
			if name, err := strconv.Unquote(lit.Value); err == nil {
				names = append(names, name)
			}
		}

		return true
	})

	return names
}

func receiverType(recv *ast.FieldList) string {
	expr := recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}

	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}

	return fmt.Sprintf("%T", expr)
}

// newType returns X for the expression new(X), and "" for anything else.
func newType(expr ast.Expr) string {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return ""
	}

	if fn, isIdent := call.Fun.(*ast.Ident); !isIdent || fn.Name != "new" {
		return ""
	}

	if ident, isIdent := call.Args[0].(*ast.Ident); isIdent {
		return ident.Name
	}

	return ""
}

func isSelector(expr ast.Expr, receiver, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}

	ident, ok := sel.X.(*ast.Ident)

	return ok && ident.Name == receiver
}

func specName(spec ast.Spec) string {
	switch spec := spec.(type) {
	case *ast.TypeSpec:
		return spec.Name.Name
	case *ast.ValueSpec:
		names := make([]string, 0, len(spec.Names))
		for _, name := range spec.Names {
			names = append(names, name.Name)
		}

		return strings.Join(names, ", ")
	default:
		return fmt.Sprintf("%T", spec)
	}
}
