package x

import "github.com/prometheus/client_golang/prometheus"

func init() { prometheus.MustRegister(nil) }
