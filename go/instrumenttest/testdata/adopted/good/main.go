package main

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrl "sigs.k8s.io/controller-runtime"

	"go.miloapis.com/instrumentation/logging"
	"go.miloapis.com/instrumentation/metrics"
)

var requests = metrics.NewCounter(prometheus.CounterOpts{Name: "requests_total"})

func main() { ctrl.SetLogger(logging.New(nil)) }
