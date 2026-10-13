package dot

import . "github.com/prometheus/client_golang/prometheus"

var c = NewCounter(CounterOpts{Name: "dot_total"})

func init() { MustRegister(c) }
