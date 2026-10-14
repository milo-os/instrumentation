// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	_ "sigs.k8s.io/controller-runtime/pkg/controller"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

func get(t *testing.T, h http.Handler) (int, string) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(body)
}

func series(body, name string) int {
	n := 0
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, name+" ") || strings.HasPrefix(line, name+"{") {
			n++
		}
	}
	return n
}

func exports(t *testing.T, g prometheus.Gatherer, name string) bool {
	t.Helper()
	families, err := g.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range families {
		if f.GetName() == name {
			return true
		}
	}
	return false
}

// The subtests run in order: the second drops controller-runtime's runtime
// collectors for the rest of the test binary.
func TestWithGatherers(t *testing.T) {
	if !exports(t, ctrlmetrics.Registry, goFamily) {
		t.Fatal("controller-runtime's registry has no Go collector, so these tests cannot see a collision")
	}
	NewCounter(prometheus.CounterOpts{Name: "instrumentation_test_ours_total", Help: "Created through this package."}).Inc()

	t.Run("keeps runtime metrics a private registry lacks", func(t *testing.T) {
		private := prometheus.NewRegistry()
		private.MustRegister(prometheus.NewGauge(prometheus.GaugeOpts{Name: "instrumentation_test_private", Help: "On a private registry."}))

		code, body := get(t, Handler(WithGatherers(private)))
		if code != http.StatusOK {
			t.Fatalf("scrape returned %d:\n%s", code, body)
		}
		for _, name := range []string{goFamily, "instrumentation_test_private", "instrumentation_test_ours_total", BuildInfoName} {
			if series(body, name) == 0 {
				t.Errorf("scrape does not contain %s", name)
			}
		}
	})

	t.Run("serves a registry that also exports runtime metrics", func(t *testing.T) {
		legacy := prometheus.NewRegistry()
		legacy.MustRegister(
			collectors.NewGoCollector(collectors.WithGoCollectorRuntimeMetrics(collectors.MetricsAll)),
			collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
			prometheus.NewGauge(prometheus.GaugeOpts{Name: "instrumentation_test_legacy", Help: "On another registry."}),
		)

		code, body := get(t, Handler(WithGatherers(legacy)))
		if code != http.StatusOK {
			t.Fatalf("scrape returned %d:\n%.2000s", code, body)
		}
		for _, name := range []string{goFamily, "instrumentation_test_legacy", "instrumentation_test_ours_total", BuildInfoName} {
			if got := series(body, name); got != 1 {
				t.Errorf("scrape has %d series of %s, want 1", got, name)
			}
		}
	})

	t.Run("fails the scrape when another registry exports one of our series", func(t *testing.T) {
		other := prometheus.NewRegistry()
		c := prometheus.NewCounter(prometheus.CounterOpts{Name: "instrumentation_test_ours_total", Help: "Created through this package."})
		c.Inc()
		other.MustRegister(c)

		code, _ := get(t, Handler(WithGatherers(other)))
		if code != http.StatusInternalServerError {
			t.Errorf("scrape returned %d, want 500 so the collision fails a test", code)
		}
	})
}
