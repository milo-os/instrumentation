package main

import (
	"log/slog"

	"github.com/go-logr/logr"
	"go.miloapis.com/instrumentation/logging"
	ctrl "sigs.k8s.io/controller-runtime"
)

func main() {
	logger := logging.New(nil)
	ctrl.SetLogger(logger)
	_ = slog.New(logr.ToSlogHandler(logger))
}
