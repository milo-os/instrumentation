package good

import "github.com/prometheus/client_golang/prometheus"

var reg = prometheus.NewRegistry()

func init() { reg.MustRegister(prometheus.NewCounter(prometheus.CounterOpts{Name: "f_total"})) }
