package bad

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var d = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "d"}, []string{"x"})
