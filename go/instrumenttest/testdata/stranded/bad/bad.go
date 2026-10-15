package bad

import (
	prom "github.com/prometheus/client_golang/prometheus"
	pa "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	a = pa.NewCounter(prom.CounterOpts{Name: "a_total"})
	b = pa.With(prom.DefaultRegisterer).NewCounter(prom.CounterOpts{Name: "b_total"})
	c = prom.NewCounter(prom.CounterOpts{Name: "c_total"})
	g = prom.DefaultGatherer
)

func init() {
	prom.MustRegister(c)
	_ = prom.Register(c)
}
