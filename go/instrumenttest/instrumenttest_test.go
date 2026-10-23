// SPDX-License-Identifier: Apache-2.0

package instrumenttest

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"go.miloapis.com/instrumentation/internal/registry"
	"go.miloapis.com/instrumentation/labels"
	"go.miloapis.com/instrumentation/metrics"
)

const (
	strandedFixture = "testdata/stranded"
	loggerFixture   = "testdata/logger"
)

func positions(findings []finding) []string {
	var out []string
	for _, f := range findings {
		out = append(out, fmt.Sprintf("%s:%d %s", f.file, f.line, f.what))
	}
	return out
}

func strandedVisit(fset *token.FileSet, file *ast.File) []finding {
	var out []finding
	for _, f := range visitMetrics(fset, file) {
		if f.kind == kindStranded || f.kind == kindDirect || f.kind == kindUnserved {
			out = append(out, f)
		}
	}
	return out
}

func TestScanFindsEveryDefaultRegistration(t *testing.T) {
	findings, scanned, err := scan(strandedFixture, newConfig([]Option{SkipDir("skipme"), SkipDir("helper")}), strandedVisit)
	if err != nil {
		t.Fatal(err)
	}
	if scanned != 9 {
		t.Errorf("scanned %d files, want 9 (bad.go, direct.go, legacy.go, plain.go, dot.go, gatherers.go, good.go, reads.go, serve.go; gen.go is go:build ignore)", scanned)
	}
	want := []string{
		"bad/bad.go:9 pa.NewCounter",
		"bad/bad.go:10 pa.With",
		"bad/bad.go:10 prom.DefaultRegisterer",
		"bad/bad.go:12 prom.DefaultGatherer",
		"bad/bad.go:16 prom.MustRegister",
		"bad/bad.go:17 prom.Register",
		"bad/direct.go:10 metrics.Registry",
		"bad/legacy.go:13 legacyregistry.MustRegister",
		"bad/legacy.go:14 legacyregistry.Registerer",
		"bad/legacy.go:15 legacyregistry.Handler",
		"bad/plain.go:8 promauto.NewGaugeVec",
		"dot/dot.go:7 MustRegister",
		"serve/serve.go:9 promhttp.Handler",
	}
	if got := positions(findings); !reflect.DeepEqual(got, want) {
		t.Errorf("findings:\n got %v\nwant %v", got, want)
	}
}

func TestSkipDirExcludesADirectory(t *testing.T) {
	findings, _, err := scan(strandedFixture, newConfig(nil), strandedVisit)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, f := range findings {
		found = found || strings.HasPrefix(f.file, "skipme/")
	}
	if !found {
		t.Error("skipme/x.go was not reported without SkipDir, so the SkipDir test proves nothing")
	}
}

func TestSkipDirRefusesADirectoryThatDoesNotExist(t *testing.T) {
	for _, dir := range []string{"renamed", "/abs/path"} {
		r := run(t, func(tb testing.TB) { assertNothingStranded(tb, strandedFixture, newConfig([]Option{SkipDir(dir)})) })
		if !r.fatal || !strings.Contains(strings.Join(r.errors, ""), "SkipDir") {
			t.Errorf("SkipDir(%q) was accepted: %v", dir, r.errors)
		}
	}
}

func TestAssertNothingStrandedReportsEachFinding(t *testing.T) {
	r := run(t, func(tb testing.TB) {
		assertNothingStranded(tb, strandedFixture, newConfig([]Option{SkipDir("skipme"), SkipDir("helper")}))
	})
	if len(r.errors) != 13 {
		t.Fatalf("got %d errors, want 13: %v", len(r.errors), r.errors)
	}
	for _, e := range r.errors {
		if !strings.Contains(e, "go.miloapis.com/instrumentation/metrics") {
			t.Errorf("error %q does not name the fix", e)
		}
	}
}

func TestAssertNothingStrandedRefusesAnEmptyModule(t *testing.T) {
	r := run(t, func(tb testing.TB) { assertNothingStranded(tb, t.TempDir(), newConfig(nil)) })
	if !r.fatal {
		t.Error("an empty module passed, so a misplaced test would pass silently")
	}
}

func TestThisModuleRegistersNothingOnTheDefaultRegistry(t *testing.T) {
	AssertNothingStranded(t, SkipDir("metrics"), SkipDir("instrumenttest"))
}

func TestRegistryPassedAsAValueIsFlagged(t *testing.T) {
	findings, _, err := scan(strandedFixture+"/helper", newConfig(nil), strandedVisit)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"helper.go:13 metrics.Registry", "helper.go:16 metrics.Registry"}
	if got := positions(findings); !reflect.DeepEqual(got, want) {
		t.Errorf("findings:\n got %v\nwant %v", got, want)
	}
}

func TestUnaliasedControllerRuntimeIsRecognized(t *testing.T) {
	r := run(t, func(tb testing.TB) { assertStandardLogger(tb, "testdata/unaliased", newConfig(nil)) })
	if len(r.errors) != 0 {
		t.Errorf("a correctly wired module was refused: %v", r.errors)
	}
}

func TestSlogBridgeIsAllowed(t *testing.T) {
	r := run(t, func(tb testing.TB) { assertStandardLogger(tb, "testdata/slogbridge", newConfig(nil)) })
	if len(r.errors) != 0 {
		t.Errorf("slog.New(logr.ToSlogHandler(logging.New(...))) was refused: %v", r.errors)
	}
}

func TestAnAddressFromConfigurationDoesNotCountAsServing(t *testing.T) {
	r := run(t, func(tb testing.TB) { assertNothingStranded(tb, "testdata/variable", newConfig(nil)) })
	if !strings.Contains(strings.Join(r.errors, "\n"), "extserver/cache.go:5: this manager's metrics server is disabled") {
		t.Errorf("a disabled server was hidden by a manager whose address comes from configuration: %v", r.errors)
	}
}

func TestADisabledMetricsServerNeedsSomethingServing(t *testing.T) {
	for _, tc := range []struct {
		skip []string
		fail bool
	}{
		{skip: []string{"serving", "handler"}, fail: true},
		{skip: []string{"handler"}, fail: false},
		{skip: []string{"serving"}, fail: false},
	} {
		var opts []Option
		for _, d := range tc.skip {
			opts = append(opts, SkipDir(d))
		}
		r := run(t, func(tb testing.TB) { assertNothingStranded(tb, "testdata/server", newConfig(opts)) })
		joined := strings.Join(r.errors, "\n")
		if got := strings.Contains(joined, "disabled/milo.go:5: this manager's metrics server is disabled"); got != tc.fail {
			t.Errorf("skipping %v: reported %v, want %v:\n%s", tc.skip, got, tc.fail, joined)
		}
	}
}

func TestAssertStandardLoggerReportsEachBypass(t *testing.T) {
	r := run(t, func(tb testing.TB) { assertStandardLogger(tb, loggerFixture, newConfig(nil)) })
	want := []string{
		"bypass/bypass.go:11: crzap.New builds a logger",
		"bypass/bypass.go:12: zap.NewProduction builds a logger",
		"bypass/bypass.go:13: zap.NewDevelopmentConfig builds a logger",
		"extserver/run.go:12: ctrl.SetLogger sets a logger, but nothing in this file calls go.miloapis.com/instrumentation/logging.New",
		"extserver/run.go:12: logr.FromSlogHandler builds a logger",
		"extserver/run.go:12: slog.NewJSONHandler builds a logger",
	}
	if len(r.errors) != len(want) {
		t.Fatalf("got %d errors, want %d: %v", len(r.errors), len(want), r.errors)
	}
	for i, w := range want {
		if !strings.HasPrefix(r.errors[i], w) {
			t.Errorf("error %d = %q, want prefix %q", i, r.errors[i], w)
		}
	}
}

func TestAssertStandardLoggerPassesTheStandardLogger(t *testing.T) {
	r := run(t, func(tb testing.TB) {
		assertStandardLogger(tb, loggerFixture, newConfig([]Option{SkipDir("bypass"), SkipDir("extserver")}))
	})
	if len(r.errors) != 0 {
		t.Errorf("errors for a module that only uses logging.New: %v", r.errors)
	}
}

func TestAssertStandardLoggerAllowsAHelperInTheSameFile(t *testing.T) {
	r := run(t, func(tb testing.TB) { assertStandardLogger(tb, "testdata/helper", newConfig(nil)) })
	if len(r.errors) != 0 {
		t.Errorf("SetLogger with a logger from logging.New in a helper in the same file was refused: %v", r.errors)
	}
}

func TestAssertStandardLoggerRefusesAModuleWithoutIt(t *testing.T) {
	r := run(t, func(tb testing.TB) { assertStandardLogger(tb, "testdata/nologger", newConfig(nil)) })
	joined := strings.Join(r.errors, "\n")
	for _, want := range []string{"x.go:5: zap.NewProduction", "calls SetLogger and builds the logger with logging.New"} {
		if !strings.Contains(joined, want) {
			t.Errorf("errors do not contain %q:\n%s", want, joined)
		}
	}
}

func TestAssertStandardLoggerRefusesALoggerThatIsNeverSet(t *testing.T) {
	r := run(t, func(tb testing.TB) { assertStandardLogger(tb, "testdata/unset", newConfig(nil)) })
	if !strings.Contains(strings.Join(r.errors, "\n"), "calls SetLogger") {
		t.Errorf("a logger built with logging.New but never set passed: %v", r.errors)
	}
}

func TestKlogSetLoggerIsRecognized(t *testing.T) {
	r := run(t, func(tb testing.TB) { assertStandardLogger(tb, "testdata/klog", newConfig(nil)) })
	want := "stray/stray.go:8: klog.SetLogger sets a logger"
	if len(r.errors) != 1 || !strings.HasPrefix(r.errors[0], want) {
		t.Errorf("errors = %v, want only %q (wired/main.go must count as wired)", r.errors, want)
	}
}

func TestSetLoggerErrorNamesTheOtherLoggingPackage(t *testing.T) {
	r := run(t, func(tb testing.TB) { assertStandardLogger(tb, "testdata/ownlogging", newConfig(nil)) })
	if !strings.Contains(strings.Join(r.errors, "\n"), "this file's logging is example.com/nso/internal/logging") {
		t.Errorf("error does not name the repository's own logging package: %v", r.errors)
	}
}

func TestDefaultImportNames(t *testing.T) {
	for path, want := range map[string]string{
		"k8s.io/klog/v2":       "klog",
		"gopkg.in/yaml.v3":     "yaml",
		"go.uber.org/zap":      "zap",
		"example.com/v2":       "example.com",
		"github.com/a/b/v10":   "b",
		"github.com/a/version": "version",
	} {
		if got := defaultName(path); got != want {
			t.Errorf("defaultName(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestAssertStandardLoggerRefusesAnEmptyModule(t *testing.T) {
	r := run(t, func(tb testing.TB) { assertStandardLogger(tb, t.TempDir(), newConfig(nil)) })
	if !r.fatal {
		t.Error("an empty module passed, so a misplaced test would pass silently")
	}
}

func TestLintPassesMetricsCreatedThroughThePackage(t *testing.T) {
	metrics.NewCounterVec(prometheus.CounterOpts{Name: "instrumenttest_good_total", Help: "Good."}, []string{labels.Outcome})
	metrics.NewHistogram(prometheus.HistogramOpts{Name: "instrumenttest_good_seconds", Help: "Good."})
	Lint(t)
}

func TestLintAfterARegistrySwap(t *testing.T) {
	create := func() {
		metrics.NewCounterVec(prometheus.CounterOpts{Name: "instrumenttest_swap_total", Help: "Swapped."}, []string{labels.Outcome})
	}
	create()
	original := ctrlmetrics.Registry
	ctrlmetrics.Registry = prometheus.NewRegistry()
	t.Cleanup(func() { ctrlmetrics.Registry = original })
	create()

	Lint(t)
}

func TestLintReportsBadNamesAndUnservedMetrics(t *testing.T) {
	bad := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "instrumenttest_bad", Help: "Bad."}, []string{"x"})
	ctrlmetrics.Registry.MustRegister(bad)
	t.Cleanup(func() { ctrlmetrics.Registry.Unregister(bad) })
	unserved := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "instrumenttest_unserved_total", Help: "Unserved."}, []string{"x"})

	r := run(t, func(tb testing.TB) {
		lint(tb, []registry.Entry{
			{Name: "instrumenttest_bad", LabelNames: []string{"x"}, Vec: bad.MetricVec},
			{Name: "instrumenttest_unserved_total", LabelNames: []string{"x"}, Vec: unserved.MetricVec},
		})
	})

	joined := strings.Join(r.errors, "\n")
	for _, want := range []string{"instrumenttest_bad: counter metrics should have \"_total\" suffix", "instrumenttest_unserved_total is not on"} {
		if !strings.Contains(joined, want) {
			t.Errorf("errors do not contain %q:\n%s", want, joined)
		}
	}
}

func TestLintRefusesWhenNoMetricsAreLinked(t *testing.T) {
	r := run(t, func(tb testing.TB) { lint(tb, nil) })
	if !r.fatal {
		t.Error("Lint passed with no metrics, so a misplaced test would pass silently")
	}
}

type recorder struct {
	testing.TB
	errors []string
	fatal  bool
}

func (r *recorder) Helper() {}

func (r *recorder) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

func (r *recorder) Fatal(args ...any) {
	r.errors = append(r.errors, fmt.Sprint(args...))
	r.fatal = true
	runtime.Goexit()
}

func (r *recorder) Fatalf(format string, args ...any) {
	r.Fatal(fmt.Sprintf(format, args...))
}

func run(t *testing.T, f func(testing.TB)) *recorder {
	r := &recorder{TB: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		f(r)
	}()
	<-done
	return r
}

func TestAssertStandardLoggerRefusesDevelopmentMode(t *testing.T) {
	r := run(t, func(tb testing.TB) { assertStandardLogger(tb, "testdata/devmode", newConfig(nil)) })
	want := []string{
		"cmd/main.go:12: Development: true turns on development mode",
		"cmd/main.go:15: UseDevMode(true) turns on development mode",
	}
	if len(r.errors) != len(want) {
		t.Fatalf("got %d errors, want %d: %v", len(r.errors), len(want), r.errors)
	}
	for i, w := range want {
		if !strings.HasPrefix(r.errors[i], w) {
			t.Errorf("error %d = %q, want prefix %q", i, r.errors[i], w)
		}
	}
}

func TestAssertAdoptedReportsBothChecksAndLinksTheGuide(t *testing.T) {
	r := run(t, func(tb testing.TB) { assertAdopted(tb, "testdata/adopted", newConfig([]Option{SkipDir("good")})) })
	joined := strings.Join(r.errors, "\n")
	for _, want := range []string{"bad/main.go:8: promauto.NewCounter", "bad/main.go:10: zap.NewProduction", "calls SetLogger"} {
		if !strings.Contains(joined, want) {
			t.Errorf("errors do not contain %q:\n%s", want, joined)
		}
	}
	for _, e := range r.errors {
		if !strings.HasSuffix(e, "; see "+guide) {
			t.Errorf("error does not link the guide: %q", e)
		}
	}
}

func TestAssertAdoptedPassesAnAdoptedModule(t *testing.T) {
	r := run(t, func(tb testing.TB) { assertAdopted(tb, "testdata/adopted", newConfig([]Option{SkipDir("bad")})) })
	if len(r.errors) != 0 || r.fatal {
		t.Errorf("an adopted module was refused: %v", r.errors)
	}
}
