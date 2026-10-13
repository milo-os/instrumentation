// SPDX-License-Identifier: Apache-2.0

// Package metrics creates Prometheus metrics that are always exported.
//
// Every constructor registers on controller-runtime's metrics registry, which
// is the registry a controller-runtime manager serves at /metrics. Metrics
// created with promauto or prometheus.MustRegister land on
// prometheus.DefaultRegisterer instead, which the manager never serves. A
// server that is not a manager serves the same registry with Handler.
//
// Use the constants in go.miloapis.com/instrumentation/labels for label
// names. Constructors panic if a label name is reserved for the collection
// pipeline or is a build label. Creating a metric again with an identical
// definition returns the existing one, so a constructor may run more than
// once. A metric name has one definition: any difference in type, label
// order, buckets, or objectives panics, including between variants with
// different constant-label values, as does creating a function metric a
// second time.
//
// Importing this package also registers instrumentation_build_info, which
// carries the build_version and instrumentation_version labels.
package metrics

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"go.miloapis.com/instrumentation/internal/registry"
	"go.miloapis.com/instrumentation/labels"
)

// HandlerOption configures Handler. None are defined yet; the parameter exists
// so options can be added without breaking callers.
type HandlerOption func(*handlerOptions)

type handlerOptions struct{}

// Handler serves the registry every constructor registers on. Use it in a
// binary whose metrics endpoint is not a controller-runtime manager's, such as
// one that sets the manager's metrics BindAddress to "0".
func Handler(_ ...HandlerOption) http.Handler {
	ensureBuildInfo(ctrlmetrics.Registry)
	return promhttp.HandlerFor(ctrlmetrics.Registry, promhttp.HandlerOpts{ErrorHandling: promhttp.HTTPErrorOnError})
}

// NewCounter creates and registers a counter.
func NewCounter(opts prometheus.CounterOpts) prometheus.Counter {
	return register(prometheus.NewCounter(opts), prometheus.Opts(opts), nil, "", nil)
}

// NewCounterVec creates and registers a counter vector.
func NewCounterVec(opts prometheus.CounterOpts, labelNames []string) *prometheus.CounterVec {
	return register(prometheus.NewCounterVec(opts, labelNames), prometheus.Opts(opts), labelNames, "", func(v *prometheus.CounterVec) *prometheus.MetricVec { return v.MetricVec })
}

// NewCounterFunc creates and registers a counter whose value fn returns at
// scrape time.
func NewCounterFunc(opts prometheus.CounterOpts, fn func() float64) prometheus.CounterFunc {
	return register(prometheus.NewCounterFunc(opts, fn), prometheus.Opts(opts), nil, funcShape, nil)
}

// NewGauge creates and registers a gauge.
func NewGauge(opts prometheus.GaugeOpts) prometheus.Gauge {
	return register(prometheus.NewGauge(opts), prometheus.Opts(opts), nil, "", nil)
}

// NewGaugeVec creates and registers a gauge vector.
func NewGaugeVec(opts prometheus.GaugeOpts, labelNames []string) *prometheus.GaugeVec {
	return register(prometheus.NewGaugeVec(opts, labelNames), prometheus.Opts(opts), labelNames, "", func(v *prometheus.GaugeVec) *prometheus.MetricVec { return v.MetricVec })
}

// NewGaugeFunc creates and registers a gauge whose value fn returns at scrape
// time.
func NewGaugeFunc(opts prometheus.GaugeOpts, fn func() float64) prometheus.GaugeFunc {
	return register(prometheus.NewGaugeFunc(opts, fn), prometheus.Opts(opts), nil, funcShape, nil)
}

// NewHistogram creates and registers a histogram.
func NewHistogram(opts prometheus.HistogramOpts) prometheus.Histogram {
	return register(prometheus.NewHistogram(opts), commonOpts(opts.Namespace, opts.Subsystem, opts.Name, opts.ConstLabels), nil, bucketShape(opts.Buckets), nil)
}

// NewHistogramVec creates and registers a histogram vector.
func NewHistogramVec(opts prometheus.HistogramOpts, labelNames []string) *prometheus.HistogramVec {
	return register(prometheus.NewHistogramVec(opts, labelNames), commonOpts(opts.Namespace, opts.Subsystem, opts.Name, opts.ConstLabels), labelNames, bucketShape(opts.Buckets), func(v *prometheus.HistogramVec) *prometheus.MetricVec { return v.MetricVec })
}

// NewSummary creates and registers a summary.
func NewSummary(opts prometheus.SummaryOpts) prometheus.Summary {
	return register(prometheus.NewSummary(opts), commonOpts(opts.Namespace, opts.Subsystem, opts.Name, opts.ConstLabels), nil, fmt.Sprint(opts.Objectives), nil)
}

// NewSummaryVec creates and registers a summary vector.
func NewSummaryVec(opts prometheus.SummaryOpts, labelNames []string) *prometheus.SummaryVec {
	return register(prometheus.NewSummaryVec(opts, labelNames), commonOpts(opts.Namespace, opts.Subsystem, opts.Name, opts.ConstLabels), labelNames, fmt.Sprint(opts.Objectives), func(v *prometheus.SummaryVec) *prometheus.MetricVec { return v.MetricVec })
}

// MustRegister registers custom collectors, such as one that computes values
// at scrape time. Their label names are not checked against labels.yaml, and
// Lint does not see them.
func MustRegister(cs ...prometheus.Collector) {
	ctrlmetrics.Registry.MustRegister(cs...)
}

func commonOpts(namespace, subsystem, name string, constLabels prometheus.Labels) prometheus.Opts {
	return prometheus.Opts{Namespace: namespace, Subsystem: subsystem, Name: name, ConstLabels: constLabels}
}

const funcShape = "func"

func bucketShape(buckets []float64) string {
	if len(buckets) == 0 {
		buckets = prometheus.DefBuckets
	}
	return fmt.Sprint(buckets)
}

type definition struct {
	kind       reflect.Type
	labelNames string
	shape      string
}

func (d definition) String() string {
	return fmt.Sprintf("%v with labels [%s] %s", d.kind, d.labelNames, d.shape)
}

// definitionKey identifies a metric name on a registry. Variants of a name
// with different constant-label values share one definition, because
// Prometheus refuses variants whose other properties differ.
type definitionKey struct {
	registry prometheus.Registerer
	name     string
}

var (
	definitionsMu sync.Mutex
	definitions   = map[definitionKey]definition{}
)

func register[T prometheus.Collector](c T, opts prometheus.Opts, labelNames []string, shape string, vec func(T) *prometheus.MetricVec) T {
	name := prometheus.BuildFQName(opts.Namespace, opts.Subsystem, opts.Name)
	for _, l := range labelNames {
		checkLabel(name, l)
	}
	for l := range opts.ConstLabels {
		checkLabel(name, l)
	}
	reg := ctrlmetrics.Registry
	ensureBuildInfo(reg)
	def := definition{kind: reflect.TypeOf(c), labelNames: strings.Join(labelNames, ","), shape: shape}
	key := definitionKey{registry: reg, name: name}

	definitionsMu.Lock()
	defer definitionsMu.Unlock()
	existing, recorded := definitions[key]
	if recorded && existing != def {
		panic(fmt.Sprintf("metric %s: already created as %s, now requested as %s", name, existing, def))
	}
	if err := reg.Register(c); err != nil {
		var already prometheus.AlreadyRegisteredError
		if !errors.As(err, &already) {
			panic(fmt.Sprintf("metric %s: %v", name, err))
		}
		if shape == funcShape {
			panic(fmt.Sprintf("metric %s: a function metric can be created only once; keep the first one", name))
		}
		collector, ok := already.ExistingCollector.(T)
		if !recorded || !ok {
			panic(fmt.Sprintf("metric %s: already registered outside go.miloapis.com/instrumentation/metrics", name))
		}
		return collector
	}
	definitions[key] = def
	entry := registry.Entry{Name: name, LabelNames: labelNames}
	if vec != nil {
		entry.Vec = vec(c)
	}
	registry.Record(reg, entry)
	return c
}

func checkLabel(metric, label string) {
	if label == labels.BuildVersion || label == labels.InstrumentationVersion {
		panic(fmt.Sprintf("metric %s: label %q would replace every series on each deploy; join on %s instead", metric, label, BuildInfoName))
	}
	if description, ok := labels.IsReserved(label); ok {
		panic(fmt.Sprintf("metric %s: label %q is reserved for the collection pipeline (%s); rename it with a constant from go.miloapis.com/instrumentation/labels, and update every prometheus.Labels map that uses the old name", metric, label, description))
	}
}
