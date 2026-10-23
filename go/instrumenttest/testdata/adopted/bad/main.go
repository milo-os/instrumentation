package main

import (
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

var requests = promauto.NewCounter

func main() { _, _ = zap.NewProduction() }
