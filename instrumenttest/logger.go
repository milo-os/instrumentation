// SPDX-License-Identifier: Apache-2.0

package instrumenttest

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"
	"testing"
)

const loggingPath = "go.miloapis.com/instrumentation/logging"

var standardLoggerRules = []rule{
	{path: loggingPath, match: oneOf("New", "NewRaw")},
}

var bypassLoggerRules = []rule{
	{path: "sigs.k8s.io/controller-runtime/pkg/log/zap", match: oneOf("New", "NewRaw")},
	{path: "go.uber.org/zap", match: oneOf("New", "NewProduction", "NewDevelopment", "NewExample", "NewProductionConfig", "NewDevelopmentConfig")},
	{path: "log/slog", match: oneOf("New", "NewJSONHandler", "NewTextHandler")},
	{path: "github.com/go-logr/logr", match: oneOf("FromSlogHandler")},
	{path: "github.com/go-logr/logr/funcr", match: oneOf("New", "NewJSON")},
}

var setLoggerRules = []rule{
	{path: "sigs.k8s.io/controller-runtime", match: oneOf("SetLogger")},
	{path: "sigs.k8s.io/controller-runtime/pkg/log", match: oneOf("SetLogger")},
	{path: "k8s.io/klog/v2", match: oneOf("SetLogger")},
}

// AssertStandardLogger fails if the current module's non-test source:
//   - builds a logger some other way, such as zap.New, slog.New, or funcr.New;
//   - calls SetLogger in a file that never calls logging.New or
//     logging.NewRaw, as a binary with its own logger would;
//   - has no file that both calls SetLogger and builds the logger with
//     logging.New or logging.NewRaw, in which case controller-runtime drops
//     every log line.
//
// Wrapping logging.NewRaw, for example with zapr.NewLogger, is allowed, as is
// bridging to slog with slog.New(logr.ToSlogHandler(logger)).
//
// The check covers the module, not each binary: a binary that never calls
// SetLogger passes as long as another file in the module is wired.
func AssertStandardLogger(t testing.TB, opts ...Option) {
	t.Helper()
	assertStandardLogger(t, currentModuleRoot(t), newConfig(opts))
}

const kindWired = "wired"

func assertStandardLogger(t testing.TB, root string, cfg config) {
	t.Helper()
	findings, scanned, err := scan(root, cfg, visitLoggers)
	if err != nil {
		t.Fatal(err)
	}
	if scanned == 0 {
		t.Fatalf("no Go source found under %s", root)
	}
	wired := false
	for _, f := range findings {
		switch {
		case f.kind == kindWired:
			wired = true
		case strings.HasSuffix(f.what, "SetLogger"):
			t.Errorf("%s:%d: %s sets a logger, but nothing in this file calls go.miloapis.com/instrumentation/logging.New%s", f.file, f.line, f.what, f.note)
		default:
			t.Errorf("%s:%d: %s builds a logger without the standard defaults; use go.miloapis.com/instrumentation/logging", f.file, f.line, f.what)
		}
	}
	if !wired {
		t.Errorf("no file in %s calls SetLogger and builds the logger with logging.New or logging.NewRaw from go.miloapis.com/instrumentation/logging; controller-runtime drops every log line until a logger is set", root)
	}
}

func visitLoggers(fset *token.FileSet, file *ast.File) []finding {
	at := func(n ast.Node, what string) finding {
		return finding{line: fset.Position(n.Pos()).Line, what: what}
	}
	var out []finding
	bridged := slogBridges(file)
	for _, r := range references(file, file, bypassLoggerRules) {
		if !bridged[r.node] {
			out = append(out, at(r.node, r.what))
		}
	}
	sets := references(file, file, setLoggerRules)
	if len(references(file, file, standardLoggerRules)) > 0 {
		if len(sets) > 0 {
			out = append(out, finding{kind: kindWired})
		}
		return out
	}
	note := otherLoggingPackage(file)
	for _, r := range sets {
		f := at(r.node, r.what)
		f.note = note
		out = append(out, f)
	}
	return out
}

// otherLoggingPackage names a different package the file imports as logging,
// such as a repository's own logging package, so the error explains why its
// logging.New does not count.
func otherLoggingPackage(file *ast.File) string {
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path == loggingPath {
			continue
		}
		name := defaultName(path)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		if name == "logging" {
			return " (this file's logging is " + path + ")"
		}
	}
	return ""
}

// slogBridges returns the slog.New references whose handler is
// logr.ToSlogHandler, which wraps a logr logger rather than building one.
func slogBridges(file *ast.File) map[ast.Node]bool {
	slogNames := importNames(file, "log/slog")
	logrNames := importNames(file, "github.com/go-logr/logr")
	bridged := map[ast.Node]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if ok && isSelector(call.Fun, slogNames, "New") && len(call.Args) == 1 && callTo(call.Args[0], logrNames, "ToSlogHandler") {
			bridged[call.Fun] = true
		}
		return true
	})
	return bridged
}
