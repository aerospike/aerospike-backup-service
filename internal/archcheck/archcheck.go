// Package archcheck holds the checks that keep the object graph honest.
//
// The service's types come in two kinds (Miško Hevery, "To new or not to new"):
//
//   - Components (injectables) are built once by the composition root, internal/app, and
//     live as long as the service does: the scheduler, the client manager, the storage
//     accessors. A component may hold other components.
//   - Newables are created at run time: configuration values, routine and job state,
//     per-cluster client entries, DTOs.
//
// A newable never holds a component in a field. A newable that needs a component takes it
// as a method argument, so the component is wired in one place and the newable can be
// built in a test from a literal.
//
// Go has no annotation that marks a component, so the composition root is the registry: a
// type is a component because of how the root wires it, not because of its name or its
// package. Check states the exact criterion.
package archcheck

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/types/typeutil"
)

// Config selects the code under check.
type Config struct {
	// Dir is the module directory the patterns are resolved in.
	Dir string
	// Root is the import path of the composition root package.
	Root string
	// Registry is the struct type in Root whose fields are the running components.
	Registry string
	// Patterns select the packages to check, e.g. "./pkg/...".
	Patterns []string
}

// Violation is a newable that holds a component in a field.
type Violation struct {
	// Holder is the field, e.g. "pkg/service/aerospike.clientInfo.factory".
	Holder string
	// Component is the component type the field holds, e.g. "aerospike.ClientFactory".
	Component string
	// Pos is "file:line" of the field, relative to the module directory.
	Pos string
}

func (v Violation) String() string {
	return fmt.Sprintf("%s: %s holds component %s; pass it to the method that needs it instead",
		v.Pos, v.Holder, v.Component)
}

// Result is the outcome of a Check.
type Result struct {
	// Components are the component types, sorted, named module-relative ("pkg/service.BackupCatalog").
	Components []string
	// Violations are sorted by Holder.
	Violations []Violation
}

// Check loads the packages matching cfg.Patterns and reports every newable that holds a
// component.
//
// The component set is read off the composition root, cfg.Root, and closed under these rules:
//
//  1. The type of every field of the registry struct, cfg.Registry, is a component.
//  2. A module interface that a call in the root returns, or that the root passes on as an
//     argument, is a component.
//  3. An interface parameter the root passes a component into is a component. This is how
//     consumer-side interfaces (BackupReader, AdHocScheduler) join the set. An interface
//     the root passes data into, such as a view over *model.Config, does not.
//  4. A type that implements a component interface is a component: the struct behind a
//     constructor that returns an interface.
//  5. A concrete module type the root gets from a call or passes on is a component if it
//     holds one: *handlers.Service.
//
// The root is not only its own package: a constructor (a package-level New* or new* function)
// it calls is part of the wiring, and so is every constructor that one calls. Rules 2, 3 and 5
// see their calls too, which is how a runner that NewRestoreManager builds joins the set.
//
// A concrete type the root handles that holds no component and implements no component
// interface (*model.Config, *dto.Config) is data, and data is newable.
//
// Every other struct type declared in the loaded packages, including types local to a
// function, is a newable. The root's own types and generated files (mocks) are not checked.
func Check(cfg Config) (Result, error) {
	pkgs, err := load(cfg)
	if err != nil {
		return Result{}, err
	}

	root := findPackage(pkgs, cfg.Root)
	if root == nil {
		return Result{}, fmt.Errorf("composition root %s is not among the loaded packages", cfg.Root)
	}

	g := newGraph(root, pkgs)
	if err := g.seed(root, cfg.Registry); err != nil {
		return Result{}, err
	}

	g.close()

	return Result{Components: g.componentNames(), Violations: g.violations(cfg.Root)}, nil
}

func load(cfg Config) ([]*packages.Package, error) {
	mode := packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes |
		packages.NeedTypesInfo | packages.NeedImports | packages.NeedModule

	pkgs, err := packages.Load(&packages.Config{Mode: mode, Dir: cfg.Dir}, cfg.Patterns...)
	if err != nil {
		return nil, fmt.Errorf("load %v: %w", cfg.Patterns, err)
	}

	var errs []error
	for _, pkg := range pkgs {
		for _, pkgErr := range pkg.Errors {
			errs = append(errs, pkgErr)
		}
	}

	return pkgs, errors.Join(errs...)
}

func findPackage(pkgs []*packages.Package, path string) *packages.Package {
	for _, pkg := range pkgs {
		if pkg.PkgPath == path {
			return pkg
		}
	}

	return nil
}

// graph is the component set as it is being closed.
type graph struct {
	module     *packages.Module
	fset       *token.FileSet
	components map[*types.TypeName]bool
	// declared are the named types in the loaded packages, generated files excluded.
	declared []*types.TypeName
	// handled are the concrete module types the root gets from calls or passes on.
	handled []*types.TypeName
	// flows pair each argument the root passes with the parameter it is passed into.
	flows []flow
	// constructors are the package-level functions of the loaded packages, by object.
	constructors map[*types.Func]constructor
	entered      map[*types.Func]bool
}

type constructor struct {
	decl *ast.FuncDecl
	info *types.Info
}

type flow struct {
	arg   types.Type
	param types.Type
}

func newGraph(root *packages.Package, pkgs []*packages.Package) *graph {
	g := &graph{
		module:       root.Module,
		fset:         root.Fset,
		components:   make(map[*types.TypeName]bool),
		constructors: make(map[*types.Func]constructor),
		entered:      make(map[*types.Func]bool),
	}

	for _, pkg := range pkgs {
		g.declared = append(g.declared, declaredTypes(pkg)...)
		g.indexConstructors(pkg)
	}

	return g
}

func (g *graph) indexConstructors(pkg *packages.Package) {
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Body == nil || !isConstructorName(fn.Name.Name) {
				continue
			}

			if obj, ok := pkg.TypesInfo.Defs[fn.Name].(*types.Func); ok {
				g.constructors[obj] = constructor{decl: fn, info: pkg.TypesInfo}
			}
		}
	}
}

func isConstructorName(name string) bool {
	return strings.HasPrefix(name, "New") || strings.HasPrefix(name, "new")
}

// declaredTypes returns every named type defined in pkg outside generated files, including
// types local to a function.
func declaredTypes(pkg *packages.Package) []*types.TypeName {
	generated := make(map[string]bool)
	for _, file := range pkg.Syntax {
		if ast.IsGenerated(file) {
			generated[pkg.Fset.File(file.Pos()).Name()] = true
		}
	}

	var named []*types.TypeName
	for _, obj := range pkg.TypesInfo.Defs {
		typeName, ok := obj.(*types.TypeName)
		if !ok || typeName.IsAlias() || generated[pkg.Fset.File(typeName.Pos()).Name()] {
			continue
		}

		if _, ok := typeName.Type().(*types.Named); ok {
			named = append(named, typeName)
		}
	}

	return named
}

func (g *graph) seed(root *packages.Package, registry string) error {
	registryType, ok := root.Types.Scope().Lookup(registry).(*types.TypeName)
	if !ok {
		return fmt.Errorf("registry %s.%s is not a type", root.PkgPath, registry)
	}

	for _, component := range namedIn(registryType.Type().Underlying()) {
		g.mark(component)
	}

	for _, file := range root.Syntax {
		g.inspect(root.TypesInfo, file)
	}

	return nil
}

// inspect records every call under node.
func (g *graph) inspect(info *types.Info, node ast.Node) {
	ast.Inspect(node, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok {
			g.call(info, call)
		}

		return true
	})
}

// call records what one call in the root hands out and takes in.
func (g *graph) call(info *types.Info, call *ast.CallExpr) {
	fn, ok := typeutil.Callee(info, call).(*types.Func)
	if !ok {
		return // a builtin, a conversion, or a call through a func value.
	}

	sig := fn.Signature()
	for result := range sig.Results().Variables() {
		g.handle(result.Type())
	}

	for i, arg := range argTypes(info, call) {
		g.handle(arg)
		g.flows = append(g.flows, flow{arg: arg, param: paramType(sig, i, call)})
	}

	g.enter(fn)
}

// enter records the calls of fn if it is a constructor of the loaded packages, once.
func (g *graph) enter(fn *types.Func) {
	ctor, ok := g.constructors[fn]
	if !ok || g.entered[fn] {
		return
	}

	g.entered[fn] = true
	g.inspect(ctor.info, ctor.decl.Body)
}

// argTypes returns the type of each argument, spreading f(g()) over g's results.
func argTypes(info *types.Info, call *ast.CallExpr) []types.Type {
	if len(call.Args) == 1 {
		if tuple, ok := info.TypeOf(call.Args[0]).(*types.Tuple); ok {
			spread := make([]types.Type, 0, tuple.Len())
			for v := range tuple.Variables() {
				spread = append(spread, v.Type())
			}

			return spread
		}
	}

	arguments := make([]types.Type, 0, len(call.Args))
	for _, arg := range call.Args {
		arguments = append(arguments, info.TypeOf(arg))
	}

	return arguments
}

// paramType returns the type argument i is passed as.
func paramType(sig *types.Signature, i int, call *ast.CallExpr) types.Type {
	params := sig.Params()
	last := params.Len() - 1
	if !sig.Variadic() || i < last {
		return params.At(i).Type()
	}

	variadic := params.At(last).Type()
	if call.Ellipsis.IsValid() {
		return variadic
	}

	return variadic.(*types.Slice).Elem()
}

// handle records a type the root gets or passes on: rule 2 for interfaces, a rule 5
// candidate for concrete types.
func (g *graph) handle(t types.Type) {
	for _, named := range valuesIn(t) {
		if !g.inModule(named) {
			continue
		}

		if isInterface(named) {
			g.mark(named)
		} else {
			g.handled = append(g.handled, named)
		}
	}
}

// close applies rules 3-5 until the component set stops growing.
func (g *graph) close() {
	for changed := true; changed; {
		// Every rule runs on each pass: a || short circuit would skip the later ones.
		byArgument := g.markParametersOfComponents()
		byImplementation := g.markImplementations()
		byHolding := g.markHandledHolders()
		changed = byArgument || byImplementation || byHolding
	}
}

// markParametersOfComponents applies rule 3.
func (g *graph) markParametersOfComponents() bool {
	changed := false

	for _, f := range g.flows {
		if !g.holds(f.arg) {
			continue
		}

		for _, named := range valuesIn(f.param) {
			if g.inModule(named) && isInterface(named) {
				changed = g.mark(named) || changed
			}
		}
	}

	return changed
}

// markImplementations applies rule 4.
func (g *graph) markImplementations() bool {
	changed := false

	for _, named := range g.declared {
		if !isInterface(named) && g.implementsComponent(named) {
			changed = g.mark(named) || changed
		}
	}

	return changed
}

// markHandledHolders applies rule 5.
func (g *graph) markHandledHolders() bool {
	changed := false

	for _, named := range g.handled {
		if g.holds(named.Type().Underlying()) {
			changed = g.mark(named) || changed
		}
	}

	return changed
}

// mark adds named to the component set and reports whether it was new.
func (g *graph) mark(named *types.TypeName) bool {
	if g.components[named] {
		return false
	}

	g.components[named] = true

	return true
}

// holds reports whether t is, or contains, a component.
func (g *graph) holds(t types.Type) bool {
	return slices.ContainsFunc(namedIn(t), func(named *types.TypeName) bool {
		return g.components[named]
	})
}

func (g *graph) implementsComponent(named *types.TypeName) bool {
	if named.Type().(*types.Named).TypeParams().Len() > 0 {
		return false // a generic type is a container, never the implementation of a service.
	}

	pointer := types.NewPointer(named.Type())

	for component := range g.components {
		iface, ok := component.Type().Underlying().(*types.Interface)
		// An empty interface is implemented by every type; the registry may name an external one.
		if !ok || iface.Empty() {
			continue
		}

		if types.Implements(named.Type(), iface) || types.Implements(pointer, iface) {
			return true
		}
	}

	return false
}

func (g *graph) violations(root string) []Violation {
	var violations []Violation

	for _, named := range g.declared {
		if named.Pkg().Path() == root || g.components[named] {
			continue
		}

		structType, ok := named.Type().Underlying().(*types.Struct)
		if !ok {
			continue
		}

		for field := range structType.Fields() {
			if component := g.componentIn(field.Type()); component != nil {
				violations = append(violations, Violation{
					Holder:    g.name(named) + "." + field.Name(),
					Component: types.TypeString(component.Type(), packageName),
					Pos:       g.position(field),
				})
			}
		}
	}

	slices.SortFunc(violations, func(a, b Violation) int { return strings.Compare(a.Holder, b.Holder) })

	return violations
}

// componentIn returns the first component t is or contains, or nil.
func (g *graph) componentIn(t types.Type) *types.TypeName {
	for _, named := range namedIn(t) {
		if g.components[named] {
			return named
		}
	}

	return nil
}

func (g *graph) componentNames() []string {
	names := make([]string, 0, len(g.components))
	for component := range g.components {
		names = append(names, g.name(component))
	}

	slices.Sort(names)

	return names
}

func (g *graph) inModule(named *types.TypeName) bool {
	pkg := named.Pkg()

	return pkg != nil && (pkg.Path() == g.module.Path || strings.HasPrefix(pkg.Path(), g.module.Path+"/"))
}

// name is the type's module-relative name, e.g. "pkg/service.BackupCatalog".
func (g *graph) name(named *types.TypeName) string {
	if named.Pkg() == nil {
		return named.Name()
	}

	return strings.TrimPrefix(named.Pkg().Path(), g.module.Path+"/") + "." + named.Name()
}

func (g *graph) position(field *types.Var) string {
	position := g.fset.Position(field.Pos())

	relative, err := filepath.Rel(g.module.Dir, position.Filename)
	if err != nil {
		relative = position.Filename
	}

	return relative + ":" + strconv.Itoa(position.Line)
}

// namedIn returns the named types t is built from: through pointers, slices, arrays, maps,
// channels, the fields of an anonymous struct, and the type arguments of a generic type. A
// named type's own fields are not entered: it is its own type.
func namedIn(t types.Type) []*types.TypeName {
	switch t := types.Unalias(t).(type) {
	case *types.Named:
		named := []*types.TypeName{t.Origin().Obj()}
		for arg := range t.TypeArgs().Types() {
			named = append(named, namedIn(arg)...)
		}

		return named
	case *types.Pointer:
		return namedIn(t.Elem())
	case *types.Slice:
		return namedIn(t.Elem())
	case *types.Array:
		return namedIn(t.Elem())
	case *types.Chan:
		return namedIn(t.Elem())
	case *types.Map:
		return append(namedIn(t.Key()), namedIn(t.Elem())...)
	case *types.Struct:
		var named []*types.TypeName
		for field := range t.Fields() {
			named = append(named, namedIn(field.Type())...)
		}

		return named
	default:
		return nil
	}
}

// valuesIn returns the named types a value of type t is, through pointers and slices: a
// constructor's []Accessor parameter takes Accessors. Unlike namedIn it does not enter maps or
// type arguments, which hold what a component creates at run time, not what it is wired with:
// the *clientInfo in NewSafeMap[uint64, *clientInfo] is newable.
func valuesIn(t types.Type) []*types.TypeName {
	switch t := types.Unalias(t).(type) {
	case *types.Named:
		return []*types.TypeName{t.Origin().Obj()}
	case *types.Pointer:
		return valuesIn(t.Elem())
	case *types.Slice:
		return valuesIn(t.Elem())
	default:
		return nil
	}
}

func isInterface(named *types.TypeName) bool {
	return types.IsInterface(named.Type())
}

func packageName(pkg *types.Package) string {
	return pkg.Name()
}
