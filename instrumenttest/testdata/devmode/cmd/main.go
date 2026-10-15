package main

import (
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"go.miloapis.com/instrumentation/logging"
)

func main() {
	opts := zap.Options{
		Development: true,
	}
	quiet := zap.Options{Development: false}
	zap.UseDevMode(true)(&opts)
	zap.UseDevMode(false)(&quiet)
	ctrl.SetLogger(logging.New(&opts))
}
