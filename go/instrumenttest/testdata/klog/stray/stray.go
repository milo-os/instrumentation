package stray

import (
	"github.com/go-logr/logr"
	"k8s.io/klog/v2"
)

func Setup(l logr.Logger) { klog.SetLogger(l) }
