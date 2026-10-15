// SPDX-License-Identifier: Apache-2.0

package instrumenttest

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil/promlint"
	dto "github.com/prometheus/client_model/go"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"go.miloapis.com/instrumentation/internal/registry"
)

const placeholder = "instrumenttest-lint"

// Lint fails if any metric created through go.miloapis.com/instrumentation/metrics
// breaks Prometheus naming conventions, as promlint checks them.
//
// It sees only metrics linked into the test binary, so call it from a package
// that imports every package defining metrics. It fails if it sees none.
//
// To lint a metric vector it briefly adds a series with every label set to
// "instrumenttest-lint", then deletes it. Do not run Lint in parallel with
// tests that count those series.
func Lint(t testing.TB) {
	t.Helper()
	lint(t, registry.Entries(ctrlmetrics.Registry))
}

func lint(t testing.TB, entries []registry.Entry) {
	t.Helper()
	if len(entries) == 0 {
		t.Fatal("no metrics created through go.miloapis.com/instrumentation/metrics are linked into this test; call Lint from a package that imports the packages defining them. If they were created with promauto or prometheus directly, AssertNothingStranded lists them")
	}

	for _, e := range entries {
		if e.Vec == nil {
			continue
		}
		values := make([]string, len(e.LabelNames))
		for i := range values {
			values[i] = placeholder
		}
		if _, err := e.Vec.GetMetricWithLabelValues(values...); err != nil {
			t.Fatalf("%s: %v", e.Name, err)
		}
		t.Cleanup(func() { e.Vec.DeleteLabelValues(values...) })
	}

	gathered, err := ctrlmetrics.Registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	byName := make(map[string]*dto.MetricFamily, len(gathered))
	for _, mf := range gathered {
		byName[mf.GetName()] = mf
	}

	var families []*dto.MetricFamily
	for _, e := range entries {
		mf, ok := byName[e.Name]
		if !ok {
			t.Errorf("%s is not on controller-runtime's metrics registry", e.Name)
			continue
		}
		families = append(families, mf)
	}

	problems, err := promlint.NewWithMetricFamilies(families).Lint()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range problems {
		t.Errorf("%s: %s", p.Metric, strings.TrimSpace(p.Text))
	}
}
