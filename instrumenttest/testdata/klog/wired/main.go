package main

import (
	"go.miloapis.com/instrumentation/logging"
	"k8s.io/klog/v2"
)

func main() { klog.SetLogger(logging.New(nil)) }
