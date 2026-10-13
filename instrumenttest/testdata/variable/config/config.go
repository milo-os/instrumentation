package config

import metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

func Options(addr string) metricsserver.Options {
	return metricsserver.Options{BindAddress: addr}
}
