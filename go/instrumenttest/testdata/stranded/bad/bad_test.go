package bad

import "github.com/prometheus/client_golang/prometheus/promauto"

var ignoredInTests = promauto.NewCounter
