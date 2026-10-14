package good

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/component-base/metrics/legacyregistry"

	"go.miloapis.com/instrumentation/metrics"
)

func init() {
	http.Handle("/metrics", metrics.Handler(metrics.WithGatherers(legacyregistry.DefaultGatherer, prometheus.DefaultGatherer)))
}
