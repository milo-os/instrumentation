// SPDX-License-Identifier: Apache-2.0

// Package registry records the metrics created through the metrics package so
// instrumenttest can inspect them.
package registry

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// Entry is one metric created through the metrics package.
type Entry struct {
	Name       string
	LabelNames []string
	Vec        *prometheus.MetricVec
}

var (
	mu      sync.Mutex
	entries = map[any][]Entry{}
)

// Record adds e to the metrics created on reg, once per name.
func Record(reg any, e Entry) {
	mu.Lock()
	defer mu.Unlock()
	for _, existing := range entries[reg] {
		if existing.Name == e.Name {
			return
		}
	}
	entries[reg] = append(entries[reg], e)
}

// Entries returns every metric created on reg so far.
func Entries(reg any) []Entry {
	mu.Lock()
	defer mu.Unlock()
	return append([]Entry(nil), entries[reg]...)
}
