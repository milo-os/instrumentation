package main

import (
	"flag"

	"go.miloapis.com/instrumentation/logging"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)
	_ = logging.New(&opts)
}

func setup(opts *zap.Options) {
	ctrl.SetLogger(logging.New(opts))
}
