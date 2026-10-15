// SPDX-License-Identifier: Apache-2.0

package instrumenttest

import (
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Option configures the source-scanning assertions.
type Option func(*config)

type config struct {
	skip map[string]bool
}

// SkipDir excludes a directory, given relative to the module root, and
// everything under it. The assertion fails if the directory does not exist,
// so a rename cannot silently widen what is skipped.
func SkipDir(dirs ...string) Option {
	return func(c *config) {
		for _, d := range dirs {
			c.skip[filepath.Clean(d)] = true
		}
	}
}

func newConfig(opts []Option) config {
	cfg := config{skip: map[string]bool{}}
	for _, o := range opts {
		o(&cfg)
	}
	return cfg
}

// rule matches identifiers of one imported package.
type rule struct {
	path  string
	match func(name string) bool
}

func oneOf(names ...string) func(string) bool {
	return func(name string) bool {
		for _, n := range names {
			if n == name {
				return true
			}
		}
		return false
	}
}

type finding struct {
	file string
	line int
	what string
	kind string
	note string
}

// ref is one use of an identifier from an imported package.
type ref struct {
	node ast.Node
	path string
	name string
	what string
}

func currentModuleRoot(t testing.TB) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod found in any parent directory")
		}
		dir = parent
	}
}

// scan runs visit on every non-test Go file under root and reports what it
// found and how many files it read.
func scan(root string, cfg config, visit func(*token.FileSet, *ast.File) []finding) ([]finding, int, error) {
	for dir := range cfg.skip {
		if filepath.IsAbs(dir) {
			return nil, 0, &skipError{dir: dir, reason: "is absolute; give it relative to the module root"}
		}
		if info, err := os.Stat(filepath.Join(root, dir)); err != nil || !info.IsDir() {
			return nil, 0, &skipError{dir: dir, reason: "is not a directory in the module"}
		}
	}

	var findings []finding
	scanned := 0
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && skipDir(path, d.Name()) || cfg.skip[rel] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution|parser.ParseComments)
		if err != nil {
			return fmt.Errorf("%w; if this directory is not meant to build, exclude it with SkipDir", err)
		}
		if ignored(file) {
			return nil
		}
		scanned++
		for _, f := range visit(fset, file) {
			f.file = filepath.ToSlash(rel)
			findings = append(findings, f)
		}
		return nil
	})
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].file != findings[j].file {
			return findings[i].file < findings[j].file
		}
		if findings[i].line != findings[j].line {
			return findings[i].line < findings[j].line
		}
		return findings[i].what < findings[j].what
	})
	return findings, scanned, err
}

// ignored reports whether the file builds only with the ignore tag, as
// generators and other go:build ignore files do.
func ignored(file *ast.File) bool {
	for _, group := range file.Comments {
		if group.Pos() >= file.Package {
			break
		}
		for _, c := range group.List {
			if !constraint.IsGoBuild(c.Text) {
				continue
			}
			expr, err := constraint.Parse(c.Text)
			if err != nil {
				return false
			}
			withoutIgnore := expr.Eval(func(tag string) bool { return tag != "ignore" })
			withEverything := expr.Eval(func(string) bool { return true })
			return withEverything && !withoutIgnore
		}
	}
	return false
}

// defaultName returns the name a package is imported under when the import
// has no explicit name: the last path element, skipping a major version
// suffix such as /v2 and dropping a gopkg.in .vN suffix.
func defaultName(path string) string {
	if name, ok := packageNames[path]; ok {
		return name
	}
	elems := strings.Split(path, "/")
	name := elems[len(elems)-1]
	if len(elems) > 1 && majorVersion(name) {
		name = elems[len(elems)-2]
	}
	if i := strings.LastIndex(name, ".v"); i > 0 && digits(name[i+2:]) {
		name = name[:i]
	}
	return name
}

// packageNames lists imported paths whose package name differs from the last
// path element.
var packageNames = map[string]string{
	"sigs.k8s.io/controller-runtime": "controllerruntime",
}

// callTo reports whether e calls a function named fn from one of the packages
// named pkgs.
func callTo(e ast.Expr, pkgs []string, fn string) bool {
	call, ok := e.(*ast.CallExpr)
	return ok && isSelector(call.Fun, pkgs, fn)
}

func majorVersion(s string) bool { return len(s) > 1 && s[0] == 'v' && digits(s[1:]) }

func digits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// importNames returns the names a file uses for an imported path.
func importNames(file *ast.File, path string) []string {
	var names []string
	for _, imp := range file.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil || p != path {
			continue
		}
		if imp.Name == nil {
			names = append(names, defaultName(path))
		} else if imp.Name.Name != "_" && imp.Name.Name != "." {
			names = append(names, imp.Name.Name)
		}
	}
	return names
}

type skipError struct {
	dir    string
	reason string
}

func (e *skipError) Error() string { return "SkipDir(" + strconv.Quote(e.dir) + ") " + e.reason }

func skipDir(path, name string) bool {
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "vendor" || name == "testdata" {
		return true
	}
	_, err := os.Stat(filepath.Join(path, "go.mod"))
	return err == nil
}

// references returns every use in node of an identifier matching rules,
// whether reached through the import's name or through a dot import.
func references(file *ast.File, node ast.Node, rules []rule) []ref {
	byName := map[string][]rule{}
	var dotted []rule
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		for _, r := range rules {
			if r.path != path {
				continue
			}
			switch {
			case imp.Name == nil:
				byName[defaultName(path)] = append(byName[defaultName(path)], r)
			case imp.Name.Name == ".":
				dotted = append(dotted, r)
			case imp.Name.Name != "_":
				byName[imp.Name.Name] = append(byName[imp.Name.Name], r)
			}
		}
	}
	if len(byName) == 0 && len(dotted) == 0 {
		return nil
	}

	var refs []ref
	notBare := map[*ast.Ident]bool{}
	ast.Inspect(node, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SelectorExpr:
			notBare[n.Sel] = true
			if pkg, ok := n.X.(*ast.Ident); ok {
				notBare[pkg] = true
				for _, r := range byName[pkg.Name] {
					if r.match(n.Sel.Name) {
						refs = append(refs, ref{node: n, path: r.path, name: n.Sel.Name, what: pkg.Name + "." + n.Sel.Name})
					}
				}
			}
		case *ast.FuncDecl:
			notBare[n.Name] = true
		case *ast.TypeSpec:
			notBare[n.Name] = true
		case *ast.ValueSpec:
			for _, id := range n.Names {
				notBare[id] = true
			}
		case *ast.Field:
			for _, id := range n.Names {
				notBare[id] = true
			}
		case *ast.KeyValueExpr:
			if id, ok := n.Key.(*ast.Ident); ok {
				notBare[id] = true
			}
		case *ast.AssignStmt:
			if n.Tok == token.DEFINE {
				for _, e := range n.Lhs {
					if id, ok := e.(*ast.Ident); ok {
						notBare[id] = true
					}
				}
			}
		}
		return true
	})
	if len(dotted) > 0 {
		ast.Inspect(node, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok || notBare[id] {
				return true
			}
			for _, r := range dotted {
				if r.match(id.Name) {
					refs = append(refs, ref{node: id, path: r.path, name: id.Name, what: id.Name})
				}
			}
			return true
		})
	}
	return refs
}
