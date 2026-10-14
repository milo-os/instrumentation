// SPDX-License-Identifier: Apache-2.0

package instrumenttest

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"go.miloapis.com/instrumentation/metrics"
)

func TestAssertScrapesPassesAWorkingHandler(t *testing.T) {
	r := run(t, func(tb testing.TB) { AssertScrapes(tb, metrics.Handler()) })
	if r.fatal || len(r.errors) != 0 {
		t.Errorf("a working handler was refused: %v", r.errors)
	}
}

func TestAssertScrapesRefusesACollision(t *testing.T) {
	metrics.NewCounter(prometheus.CounterOpts{Name: "instrumenttest_collision_total", Help: "Ours."}).Inc()
	other := prometheus.NewRegistry()
	c := prometheus.NewCounter(prometheus.CounterOpts{Name: "instrumenttest_collision_total", Help: "Ours."})
	c.Inc()
	other.MustRegister(c)

	r := run(t, func(tb testing.TB) { AssertScrapes(tb, metrics.Handler(metrics.WithGatherers(other))) })
	if !strings.Contains(strings.Join(r.errors, "\n"), "scrape returned 500") {
		t.Errorf("a series exported by two registries passed: %v", r.errors)
	}
}
