// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"go.miloapis.com/instrumentation/buildinfo"
	"go.miloapis.com/instrumentation/labels"
)

func TestMetricsAreServedByControllerRuntime(t *testing.T) {
	served := NewCounterVec(prometheus.CounterOpts{
		Name: "instrumentation_test_served_total",
		Help: "Created through this package.",
	}, []string{labels.Project})
	served.WithLabelValues("p1").Inc()

	stranded := promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "instrumentation_test_stranded_total",
		Help: "Created on the default registry.",
	}, []string{labels.Project})
	stranded.WithLabelValues("p1").Inc()

	body := scrape(t)

	if !strings.Contains(body, `instrumentation_test_served_total{project="p1"} 1`) {
		t.Errorf("metric created through this package is not served:\n%s", body)
	}
	if strings.Contains(body, "instrumentation_test_stranded_total") {
		t.Error("metric on the default registry is served, so this test cannot tell the two apart")
	}
}

func TestBuildInfoIsServed(t *testing.T) {
	body := scrape(t)

	want := fmt.Sprintf(`instrumentation_build_info{build_version="%s",instrumentation_version="%s"} 1`, buildinfo.Version(), buildinfo.InstrumentationVersion())
	if !strings.Contains(body, want) {
		t.Errorf("scrape does not contain %s:\n%s", want, body)
	}
}

func TestBuildInfoToleratesASecondRegistration(t *testing.T) {
	registerBuildInfo(ctrlmetrics.Registry)
}

func TestHandlerServesTheSameRegistry(t *testing.T) {
	NewGaugeFunc(prometheus.GaugeOpts{Name: "instrumentation_test_handler", Help: "Served by Handler."}, func() float64 { return 7 })
	NewCounterFunc(prometheus.CounterOpts{Name: "instrumentation_test_func_total", Help: "Counter func."}, func() float64 { return 3 })
	NewSummary(prometheus.SummaryOpts{Name: "instrumentation_test_summary_seconds", Help: "Summary."}).Observe(1)
	NewSummaryVec(prometheus.SummaryOpts{Name: "instrumentation_test_summary_vec_seconds", Help: "Summary vec."}, []string{labels.Outcome}).WithLabelValues("success").Observe(1)

	srv := httptest.NewServer(Handler())
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
	for _, want := range []string{
		"instrumentation_test_handler 7",
		"instrumentation_test_func_total 3",
		"instrumentation_test_summary_seconds_count 1",
		`instrumentation_test_summary_vec_seconds_count{outcome="success"} 1`,
		BuildInfoName,
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("Handler output does not contain %q", want)
		}
	}
}

func TestConstructorsReadTheCurrentRegistry(t *testing.T) {
	original := ctrlmetrics.Registry
	swapped := prometheus.NewRegistry()
	ctrlmetrics.Registry = swapped
	t.Cleanup(func() { ctrlmetrics.Registry = original })

	NewCounter(prometheus.CounterOpts{Name: "instrumentation_test_swapped_total", Help: "On the swapped registry."}).Inc()

	families, err := swapped.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range families {
		names = append(names, f.GetName())
	}
	if want := []string{BuildInfoName, "instrumentation_test_swapped_total"}; !reflect.DeepEqual(names, want) {
		t.Errorf("swapped registry has %v, want %v", names, want)
	}
}

func TestCreatingAMetricTwiceReturnsTheExistingOne(t *testing.T) {
	opts := prometheus.CounterOpts{Name: "instrumentation_test_twice_total", Help: "Created twice."}
	first := NewCounterVec(opts, []string{labels.Project, labels.Outcome})
	second := NewCounterVec(opts, []string{labels.Project, labels.Outcome})
	if first != second {
		t.Error("second constructor call returned a different vector, so increments would be lost")
	}
}

func TestEquivalentDefinitionsAreAccepted(t *testing.T) {
	a := NewCounterVec(prometheus.CounterOpts{Name: "instrumentation_test_const_total", Help: "x", ConstLabels: prometheus.Labels{"controller": "a"}}, []string{labels.Outcome})
	NewCounterVec(prometheus.CounterOpts{Name: "instrumentation_test_const_total", Help: "x", ConstLabels: prometheus.Labels{"controller": "b"}}, []string{labels.Outcome})
	if again := NewCounterVec(prometheus.CounterOpts{Name: "instrumentation_test_const_total", Help: "x", ConstLabels: prometheus.Labels{"controller": "a"}}, []string{labels.Outcome}); again != a {
		t.Error("recreating the first constant-label variant returned a different vector")
	}

	NewHistogram(prometheus.HistogramOpts{Name: "instrumentation_test_default_buckets_seconds", Help: "x"})
	NewHistogram(prometheus.HistogramOpts{Name: "instrumentation_test_default_buckets_seconds", Help: "x", Buckets: []float64{}})
}

func TestAMismatchPanicNamesBothDefinitions(t *testing.T) {
	NewCounterVec(prometheus.CounterOpts{Name: "instrumentation_test_message_total", Help: "x"}, []string{labels.Project, labels.Outcome})
	defer func() {
		msg := fmt.Sprint(recover())
		if !strings.Contains(msg, "[project,outcome]") || !strings.Contains(msg, "[outcome,project]") {
			t.Errorf("panic does not show both label orders: %s", msg)
		}
	}()
	NewCounterVec(prometheus.CounterOpts{Name: "instrumentation_test_message_total", Help: "x"}, []string{labels.Outcome, labels.Project})
}

func TestVariantsWithDifferentConstantLabelsShareOneDefinition(t *testing.T) {
	NewCounter(prometheus.CounterOpts{Name: "instrumentation_test_variant_kind", Help: "x", ConstLabels: prometheus.Labels{"controller": "a"}})
	NewHistogram(prometheus.HistogramOpts{Name: "instrumentation_test_variant_seconds", Help: "x", ConstLabels: prometheus.Labels{"controller": "a"}})

	for name, create := range map[string]func(){
		"kind": func() {
			NewGauge(prometheus.GaugeOpts{Name: "instrumentation_test_variant_kind", Help: "x", ConstLabels: prometheus.Labels{"controller": "b"}})
		},
		"buckets": func() {
			NewHistogram(prometheus.HistogramOpts{Name: "instrumentation_test_variant_seconds", Help: "x", Buckets: []float64{1, 2}, ConstLabels: prometheus.Labels{"controller": "b"}})
		},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("accepted")
				}
			}()
			create()
		})
	}

	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("scrape returned %d: %s", rec.Code, rec.Body.String())
	}
}

func TestADifferentDefinitionPanics(t *testing.T) {
	NewCounterVec(prometheus.CounterOpts{Name: "instrumentation_test_order_total", Help: "x"}, []string{labels.Project, labels.Outcome})
	NewHistogram(prometheus.HistogramOpts{Name: "instrumentation_test_buckets_seconds", Help: "x", Buckets: []float64{1, 2}})
	NewGauge(prometheus.GaugeOpts{Name: "instrumentation_test_kind", Help: "x"})
	NewGaugeFunc(prometheus.GaugeOpts{Name: "instrumentation_test_func", Help: "x"}, func() float64 { return 1 })
	unmanaged := prometheus.NewGauge(prometheus.GaugeOpts{Name: "instrumentation_test_unmanaged", Help: "x"})
	ctrlmetrics.Registry.MustRegister(unmanaged)
	t.Cleanup(func() { ctrlmetrics.Registry.Unregister(unmanaged) })

	for name, create := range map[string]func(){
		"label order": func() {
			NewCounterVec(prometheus.CounterOpts{Name: "instrumentation_test_order_total", Help: "x"}, []string{labels.Outcome, labels.Project})
		},
		"buckets": func() {
			NewHistogram(prometheus.HistogramOpts{Name: "instrumentation_test_buckets_seconds", Help: "x", Buckets: []float64{1, 5}})
		},
		"kind": func() {
			NewCounter(prometheus.CounterOpts{Name: "instrumentation_test_kind", Help: "x"})
		},
		"function twice": func() {
			NewGaugeFunc(prometheus.GaugeOpts{Name: "instrumentation_test_func", Help: "x"}, func() float64 { return 2 })
		},
		"registered elsewhere": func() {
			NewGauge(prometheus.GaugeOpts{Name: "instrumentation_test_unmanaged", Help: "x"})
		},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("accepted")
				}
			}()
			create()
		})
	}
}

func TestBuildLabelsAreRefused(t *testing.T) {
	for _, label := range []string{labels.BuildVersion, labels.InstrumentationVersion} {
		t.Run(label, func(t *testing.T) {
			defer func() {
				if msg := fmt.Sprint(recover()); !strings.Contains(msg, BuildInfoName) {
					t.Errorf("panic %q does not point to %s", msg, BuildInfoName)
				}
			}()
			NewGaugeVec(prometheus.GaugeOpts{Name: "instrumentation_test_build_" + label}, []string{label})
		})
	}
}

func TestReservedLabelsAreRefused(t *testing.T) {
	reserved := labels.Reserved()
	if len(reserved) == 0 {
		t.Fatal("no reserved labels generated from labels.yaml")
	}
	for _, label := range reserved {
		t.Run(label, func(t *testing.T) {
			name := "instrumentation_test_reserved_" + label
			wantPanic(t, label, func() {
				NewGaugeVec(prometheus.GaugeOpts{Name: name}, []string{label})
			})
			wantPanic(t, label, func() {
				NewGauge(prometheus.GaugeOpts{Name: name + "_const", ConstLabels: prometheus.Labels{label: "x"}})
			})
		})
	}
}

func TestSubjectLabelsAreAccepted(t *testing.T) {
	for i, label := range []string{labels.Organization, labels.Project, labels.ResourceKind, labels.ResourceNamespace, labels.ResourceName, labels.Reason, labels.Outcome} {
		NewHistogramVec(prometheus.HistogramOpts{Name: fmt.Sprintf("instrumentation_test_subject_%d_seconds", i)}, []string{label})
	}
}

func wantPanic(t *testing.T, label string, f func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("label %q was accepted", label)
			return
		}
		if msg := fmt.Sprint(r); !strings.Contains(msg, "reserved") {
			t.Errorf("panic %q does not explain the label is reserved", msg)
		}
	}()
	f()
}

func scrape(t *testing.T) string {
	t.Helper()
	srv, err := metricsserver.NewServer(metricsserver.Options{BindAddress: "127.0.0.1:0"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Start(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	})

	addr := srv.(interface{ GetBindAddr() string })
	deadline := time.Now().Add(10 * time.Second)
	for addr.GetBindAddr() == "" {
		if time.Now().After(deadline) {
			t.Fatal("metrics server did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}

	resp, err := http.Get("http://" + addr.GetBindAddr() + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /metrics: %s\n%s", resp.Status, body)
	}
	return string(body)
}
