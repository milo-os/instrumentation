package main

import (
	"sigs.k8s.io/controller-runtime"

	"go.miloapis.com/instrumentation/logging"
)

func main() { controllerruntime.SetLogger(logging.New(nil)) }
