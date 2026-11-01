// Package code loads the Go packages of a module and checks them against an
// architecture model.
package code

import (
	"cmp"
	"errors"
	"fmt"
	"go/ast"
	"go/types"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/packages"
)

// maxLoadErrors bounds the package errors that Load reports.
const maxLoadErrors = 10

// A Graph is the Go code of one module, as the checks see it.
type Graph struct {
	// Module is the path of the module.
	Module string
	// Packages are the packages of the module, each with its test files.
	Packages []Package
	// Unread are the paths that match the pattern ./..., but that the checks
	// cannot read: Go files that build constraints exclude or that use cgo,
	// and directories that cannot be listed.
	Unread []string
}

// A Package is one package of the module.
type Package struct {
	// Path is the import path. The files of an external test package belong
	// to the package that they test.
	Path string
	// Files are the Go files of the package, with its test files.
	Files []File
}

// A File is one Go file of a package.
type File struct {
	// Name is the path of the file, relative to the module root, with
	// forward slashes.
	Name string
	// Test reports whether the file is a test file.
	Test bool
	// Imports are the import declarations of the file.
	Imports []Import
	// Symbols are the uses of symbols from other packages.
	Symbols []Symbol
}

// An Import is one import declaration of a file.
type Import struct {
	// Path is the import path.
	Path string
	// Module is the path of the module of the imported package. It is empty
	// for a package of the standard library.
	Module string
	// Line is the line of the declaration.
	Line int
}

// A Symbol is one use of a package-level function or variable of another
// package, or of a builtin function.
type Symbol struct {
	// Name is "importpath.Name", or the name of a builtin function.
	Name string
	// Line is the line of the use.
	Line int
}

// Load loads every package of the Go module in dir, with its test files. A
// module with no package gives a graph with no packages.
func Load(dir string) (Graph, error) {
	modPath, err := modulePath(dir)
	if err != nil {
		return Graph{}, err
	}
	deps, err := load(dir, packages.NeedName|packages.NeedImports|packages.NeedDeps|packages.NeedModule)
	if err != nil {
		return Graph{}, err
	}
	roots, err := load(dir, packages.NeedName|packages.NeedFiles|packages.NeedSyntax|packages.NeedTypes|
		packages.NeedTypesInfo|packages.NeedModule|packages.NeedForTest)
	if err != nil {
		return Graph{}, err
	}
	root, err := filepath.Abs(dir)
	if err != nil {
		return Graph{}, fmt.Errorf("resolve %s: %w", dir, err)
	}
	return buildGraph(modPath, root, roots, modulePaths(deps)), nil
}

// modulePath reads the module path from the go.mod file in dir. It reads the
// file through an os.Root, so the read stays inside dir.
func modulePath(dir string) (string, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return "", fmt.Errorf("open the module directory: %w", err)
	}
	content, err := root.ReadFile("go.mod")
	if closeErr := root.Close(); err == nil && closeErr != nil {
		err = closeErr
	}
	if err != nil {
		return "", fmt.Errorf("read the go.mod file: %w", err)
	}
	mod := modfile.ModulePath(content)
	if mod == "" {
		return "", fmt.Errorf("the go.mod file in %s states no module path", dir)
	}
	return mod, nil
}

// load loads the packages of the module in dir that match the pattern ./...,
// with their test variants. It fails when a package has an error.
func load(dir string, mode packages.LoadMode) ([]*packages.Package, error) {
	roots, err := packages.Load(&packages.Config{Mode: mode, Dir: dir, Tests: true}, "./...")
	if err == nil {
		err = packageErrors(roots)
	}
	if err != nil {
		return nil, fmt.Errorf("load the packages in %s: %w", dir, err)
	}
	return roots, nil
}

// packageErrors joins the errors of the packages and of their imports. It
// reports maxLoadErrors of them, and then the number of the others. The go
// command reports a pattern that matches no package as an error of a
// package with no files, which is not an error here.
func packageErrors(roots []*packages.Package) error {
	var errs []error
	packages.Visit(roots, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			if len(p.GoFiles) == 0 && len(p.CompiledGoFiles) == 0 && strings.Contains(e.Msg, "matched no packages") {
				continue
			}
			errs = append(errs, e)
		}
	})
	if len(errs) > maxLoadErrors {
		errs = append(errs[:maxLoadErrors], fmt.Errorf("and %d more errors", len(errs)-maxLoadErrors))
	}
	return errors.Join(errs...)
}

// modulePaths maps the import path of each package in the import graph of the
// roots to the path of its module. A package of the standard library has no
// module, so it has no entry.
func modulePaths(roots []*packages.Package) map[string]string {
	modules := make(map[string]string)
	packages.Visit(roots, nil, func(p *packages.Package) {
		if p.Module != nil {
			modules[p.PkgPath] = p.Module.Path
		}
	})
	return modules
}

// buildGraph merges the package variants that go/packages returns for tests
// into one Package for each import path. A test variant belongs to the
// package that it tests. A file outside the module root, such as the
// generated main file of a test binary, is not part of the graph.
func buildGraph(modPath, root string, roots []*packages.Package, modules map[string]string) Graph {
	byPath := make(map[string]*Package)
	read := make(map[string]bool)
	for _, p := range roots {
		owner := cmp.Or(p.ForTest, p.PkgPath)
		for _, syntax := range p.Syntax {
			abs := p.Fset.File(syntax.FileStart).Name()
			name, ok := strings.CutPrefix(abs, root+string(filepath.Separator))
			if !ok {
				continue
			}
			name = filepath.ToSlash(name)
			if read[name] {
				continue
			}
			read[name] = true
			gp := byPath[owner]
			if gp == nil {
				gp = &Package{Path: owner}
				byPath[owner] = gp
			}
			gp.Files = append(gp.Files, fileOf(p, syntax, name, modules))
		}
	}
	g := Graph{Module: modPath, Unread: unread(os.DirFS(root), ".", read)}
	for _, owner := range slices.Sorted(maps.Keys(byPath)) {
		gp := byPath[owner]
		slices.SortFunc(gp.Files, func(x, y File) int { return cmp.Compare(x.Name, y.Name) })
		g.Packages = append(g.Packages, *gp)
	}
	return g
}

// unread returns the Go files under dir that match the pattern ./..., but
// that are not in read, and each directory that fsys cannot list. Like the
// go command, it skips the directories named vendor or testdata, the files
// and directories whose names begin with "." or "_", and the directories of
// other modules.
func unread(fsys fs.FS, dir string, read map[string]bool) []string {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return []string{dir}
	}
	var paths []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") || strings.HasPrefix(e.Name(), "_") {
			continue
		}
		name := path.Join(dir, e.Name())
		switch {
		case e.IsDir():
			if e.Name() != "vendor" && e.Name() != "testdata" && !isModule(fsys, name) {
				paths = append(paths, unread(fsys, name, read)...)
			}
		case strings.HasSuffix(e.Name(), ".go") && !read[name]:
			paths = append(paths, name)
		}
	}
	return paths
}

// isModule reports whether directory dir of fsys holds a go.mod file.
func isModule(fsys fs.FS, dir string) bool {
	_, err := fs.Stat(fsys, path.Join(dir, "go.mod"))
	return err == nil
}

func fileOf(p *packages.Package, syntax *ast.File, name string, modules map[string]string) File {
	f := File{Name: name, Test: strings.HasSuffix(name, "_test.go")}
	for _, spec := range syntax.Imports {
		importPath := p.TypesInfo.PkgNameOf(spec).Imported().Path()
		f.Imports = append(f.Imports, Import{
			Path:   importPath,
			Module: modules[importPath],
			Line:   p.Fset.Position(spec.Pos()).Line,
		})
	}
	ast.Inspect(syntax, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			if symbol := symbolName(p.TypesInfo.Uses[ident], p.Types); symbol != "" {
				f.Symbols = append(f.Symbols, Symbol{Name: symbol, Line: p.Fset.Position(ident.Pos()).Line})
			}
		}
		return true
	})
	return f
}

// symbolName returns the name of a builtin function, or the "importpath.Name"
// of a package-level function or variable of another package. It returns the
// empty string for every other object. A method of a predeclared type, such
// as error.Error, has no package. For a nil package, Scope returns the
// universe scope, which does not hold the method.
func symbolName(obj types.Object, self *types.Package) string {
	switch o := obj.(type) {
	case *types.Builtin:
		return o.Name()
	case *types.Func, *types.Var:
		if pkg := o.Pkg(); pkg != self && pkg.Scope().Lookup(o.Name()) == o {
			return pkg.Path() + "." + o.Name()
		}
	}
	return ""
}
