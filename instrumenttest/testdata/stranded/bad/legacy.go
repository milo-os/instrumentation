package bad

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/component-base/metrics/legacyregistry"
)

var authz = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "authz_total"}, []string{"decision"})

func init() {
	legacyregistry.MustRegister(authz)
	legacyregistry.Registerer().MustRegister(authz)
	http.Handle("/metrics", legacyregistry.Handler())
}
