package helper

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var c = prometheus.NewCounter(prometheus.CounterOpts{Name: "c_total"})

func registerMetrics(reg prometheus.Registerer) error { return reg.Register(c) }

func Start() error {
	if err := registerMetrics(metrics.Registry); err != nil {
		return err
	}
	reg := metrics.Registry
	reg.MustRegister(c)
	return nil
}
