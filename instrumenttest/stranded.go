// SPDX-License-Identifier: Apache-2.0

// Package instrumenttest provides test assertions that prove a service's
// instrumentation is wired up, not just defined.
package instrumenttest

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"
	"testing"
)

const (
	kindStranded = "stranded"
	kindDirect   = "direct"
	kindDisabled = "disabled"
	kindServing  = "serving"
	kindHandler  = "handler"

	metricsServerPath = "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var strandedRules = []rule{
	{
		path:  "github.com/prometheus/client_golang/prometheus/promauto",
		match: func(name string) bool { return strings.HasPrefix(name, "New") },
	},
	{
		path:  "github.com/prometheus/client_golang/prometheus",
		match: oneOf("MustRegister", "Register", "DefaultRegisterer", "DefaultGatherer"),
	},
	{
		path:  "github.com/prometheus/client_golang/prometheus/promhttp",
		match: oneOf("Handler"),
	},
}

var directRules = []rule{
	{path: "github.com/prometheus/client_golang/prometheus/promauto", match: oneOf("With")},
}

const (
	ctrlMetricsPath = "sigs.k8s.io/controller-runtime/pkg/metrics"
	promhttpPath    = "github.com/prometheus/client_golang/prometheus/promhttp"
)

var handlerRules = []rule{
	{path: "go.miloapis.com/instrumentation/metrics", match: oneOf("Handler")},
}

// AssertNothingStranded fails for every place in the current module's
// non-test source where a metric would not be exported, or would skip the
// checks in go.miloapis.com/instrumentation/metrics:
//   - a use of prometheus.DefaultRegisterer, which a controller-runtime
//     manager never serves;
//   - any use of controller-runtime's registry other than reading it with
//     Gather or serving it with promhttp.HandlerFor, and promauto.With, since
//     registering through them skips the reserved-label check and Lint;
//   - a manager whose metrics server is disabled with BindAddress "0" when
//     nothing in the module serves metrics: no metrics.Handler, and no
//     metricsserver.Options with a literal address or none at all. An address
//     from a variable or constant counts as neither, so a module that sets
//     its address from configuration must serve metrics.Handler or
//     SkipDir the disabled manager.
//
// It reads source rather than inspecting registries at runtime, so it finds
// every such use in this module wherever the test lives and in whatever order
// packages initialize. It cannot see registrations made inside dependencies,
// and it only recognizes BindAddress "0" written as a string literal.
func AssertNothingStranded(t testing.TB, opts ...Option) {
	t.Helper()
	assertNothingStranded(t, currentModuleRoot(t), newConfig(opts))
}

func assertNothingStranded(t testing.TB, root string, cfg config) {
	t.Helper()
	findings, scanned, err := scan(root, cfg, visitMetrics)
	if err != nil {
		t.Fatal(err)
	}
	if scanned == 0 {
		t.Fatalf("no Go source found under %s", root)
	}
	served := false
	for _, f := range findings {
		served = served || f.kind == kindServing || f.kind == kindHandler
	}
	for _, f := range findings {
		switch f.kind {
		case kindStranded:
			t.Errorf("%s:%d: %s uses prometheus.DefaultRegisterer, which a controller-runtime manager never serves; create metrics with go.miloapis.com/instrumentation/metrics", f.file, f.line, f.what)
		case kindDirect:
			t.Errorf("%s:%d: %s uses controller-runtime's registry directly, so metrics registered through it skip the reserved-label check and Lint; use the go.miloapis.com/instrumentation/metrics constructors or metrics.MustRegister, and metrics.Handler to serve it", f.file, f.line, f.what)
		case kindDisabled:
			if !served {
				t.Errorf("%s:%d: this manager's metrics server is disabled and nothing in the module serves metrics.Handler, so no metric is exported; mount metrics.Handler on the server that serves /metrics", f.file, f.line)
			}
		}
	}
}

func visitMetrics(fset *token.FileSet, file *ast.File) []finding {
	at := func(n ast.Node, what, kind string) finding {
		return finding{line: fset.Position(n.Pos()).Line, what: what, kind: kind}
	}
	var out []finding
	for _, r := range references(file, file, strandedRules) {
		out = append(out, at(r.node, r.what, kindStranded))
	}
	for _, r := range references(file, file, directRules) {
		out = append(out, at(r.node, r.what, kindDirect))
	}
	ctrlNames := importNames(file, ctrlMetricsPath)
	promhttpNames := importNames(file, promhttpPath)
	allowed := map[ast.Node]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SelectorExpr:
			if n.Sel.Name == "Gather" && isSelector(n.X, ctrlNames, "Registry") {
				allowed[n.X] = true
			}
		case *ast.CallExpr:
			if isSelector(n.Fun, promhttpNames, "HandlerFor") && len(n.Args) > 0 {
				allowed[n.Args[0]] = true
			}
		}
		return true
	})
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if ok && !allowed[sel] && isSelector(sel, ctrlNames, "Registry") {
			out = append(out, at(sel, sel.X.(*ast.Ident).Name+".Registry", kindDirect))
		}
		return true
	})
	for _, r := range references(file, file, handlerRules) {
		out = append(out, at(r.node, r.what, kindHandler))
	}
	names := importNames(file, metricsServerPath)
	if len(names) == 0 {
		return out
	}
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok || !isSelector(lit.Type, names, "Options") {
			return true
		}
		switch bindAddress(lit) {
		case bindDisabled:
			out = append(out, at(lit, "metricsserver.Options", kindDisabled))
		case bindServing:
			out = append(out, at(lit, "metricsserver.Options", kindServing))
		}
		return true
	})
	return out
}

func isSelector(e ast.Expr, pkgs []string, name string) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	for _, p := range pkgs {
		if id.Name == p {
			return true
		}
	}
	return false
}

type bind int

const (
	bindUnknown bind = iota
	bindServing
	bindDisabled
)

// bindAddress classifies a metricsserver.Options literal. A BindAddress set
// from a variable or constant is unknown, so it neither serves nor disables.
func bindAddress(lit *ast.CompositeLit) bind {
	for _, e := range lit.Elts {
		kv, ok := e.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok || key.Name != "BindAddress" {
			continue
		}
		val, ok := kv.Value.(*ast.BasicLit)
		if !ok || val.Kind != token.STRING {
			return bindUnknown
		}
		addr, err := strconv.Unquote(val.Value)
		switch {
		case err != nil:
			return bindUnknown
		case addr == "0":
			return bindDisabled
		default:
			return bindServing
		}
	}
	return bindServing
}
