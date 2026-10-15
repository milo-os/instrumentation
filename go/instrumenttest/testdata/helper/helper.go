package helper

import (
	"github.com/go-logr/logr"
	"go.miloapis.com/instrumentation/logging"
	ctrl "sigs.k8s.io/controller-runtime"
)

func newLogger() logr.Logger { return logging.New(nil) }

func Setup() { ctrl.SetLogger(newLogger()) }
