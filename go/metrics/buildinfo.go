// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"errors"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"go.miloapis.com/instrumentation/buildinfo"
	"go.miloapis.com/instrumentation/labels"
)

// BuildInfoName is the info metric this package registers. Join on it to
// break a query down by version without putting the version on every series.
const BuildInfoName = "instrumentation_build_info"

var buildInfoDesc = prometheus.NewDesc(
	BuildInfoName,
	"Always 1. Labels name the binary's version and the instrumentation library version.",
	[]string{labels.BuildVersion, labels.InstrumentationVersion},
	nil,
)

type buildInfoCollector struct{}

func (buildInfoCollector) Describe(ch chan<- *prometheus.Desc) { ch <- buildInfoDesc }

func (buildInfoCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(buildInfoDesc, prometheus.GaugeValue, 1,
		buildinfo.Version(), buildinfo.InstrumentationVersion())
}

var (
	buildInfoMu         sync.Mutex
	buildInfoRegistries = map[prometheus.Registerer]bool{}
)

func init() {
	ensureBuildInfo(ctrlmetrics.Registry)
}

// ensureBuildInfo registers instrumentation_build_info on r once, so a
// replaced controller-runtime registry gets it too.
func ensureBuildInfo(r prometheus.Registerer) {
	buildInfoMu.Lock()
	defer buildInfoMu.Unlock()
	if buildInfoRegistries[r] {
		return
	}
	registerBuildInfo(r)
	buildInfoRegistries[r] = true
}

func registerBuildInfo(r prometheus.Registerer) {
	err := r.Register(buildInfoCollector{})
	var already prometheus.AlreadyRegisteredError
	if err != nil && !errors.As(err, &already) {
		panic(err)
	}
}
