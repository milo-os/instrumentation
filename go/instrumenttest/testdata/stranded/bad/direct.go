package bad

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var direct = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "direct_total"}, []string{"cluster"})

func init() { metrics.Registry.MustRegister(direct) }
