//go:build ignore

package main

import "github.com/prometheus/client_golang/prometheus/promauto"

var ignored = promauto.NewCounter
