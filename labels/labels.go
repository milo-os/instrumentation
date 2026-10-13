// SPDX-License-Identifier: Apache-2.0

// Package labels is the shared vocabulary of label names for metrics and logs.
//
// Subject labels, such as Project, describe what a signal is about. Use the
// same constant as a metric label name and as a log key, so a metric and the
// log lines about the same resource can be joined.
//
// Reserved labels, such as cluster and namespace, are set by the collection
// pipeline on every metric and log stream. Code that sets one collides with
// the pipeline's value: Prometheus renames the metric label to
// exported_<name>, and Loki renames the log field to <name>_extracted.
//
// The names come from labels.yaml at the repository root.
package labels

//go:generate go run ../internal/labelgen -in ../labels.yaml -out zz_generated.go -package labels

import "sort"

// IsReserved reports whether name is set by the collection pipeline, and if so
// what it means.
func IsReserved(name string) (description string, ok bool) {
	description, ok = reserved[name]
	return description, ok
}

// Reserved returns every reserved label name, sorted.
func Reserved() []string {
	names := make([]string, 0, len(reserved))
	for name := range reserved {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
