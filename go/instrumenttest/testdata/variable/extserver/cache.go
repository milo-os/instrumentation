package extserver

import metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

var opts = metricsserver.Options{BindAddress: "0"}
