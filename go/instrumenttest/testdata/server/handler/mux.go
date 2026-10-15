package handler

import (
	"net/http"

	"go.miloapis.com/instrumentation/metrics"
)

func init() { http.Handle("/metrics", metrics.Handler()) }
