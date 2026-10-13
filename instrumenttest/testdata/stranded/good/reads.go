package good

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var _ = promhttp.HandlerFor(ctrlmetrics.Registry, promhttp.HandlerOpts{})

func gather() { _, _ = ctrlmetrics.Registry.Gather(); _ = http.NewServeMux() }
