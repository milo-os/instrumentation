// SPDX-License-Identifier: Apache-2.0

package labels

import (
	"sort"
	"testing"
)

func TestSubjectLabelsAreNotReserved(t *testing.T) {
	for _, name := range []string{Project, ResourceKind, ResourceNamespace, ResourceName, Reason, Outcome} {
		if _, ok := IsReserved(name); ok {
			t.Errorf("subject label %q is also reserved", name)
		}
	}
}

func TestReservedIncludesPipelineLabels(t *testing.T) {
	names := Reserved()
	if !sort.StringsAreSorted(names) {
		t.Errorf("Reserved() is not sorted: %v", names)
	}
	for _, name := range []string{"cluster", "namespace", "pod", "job", "instance"} {
		description, ok := IsReserved(name)
		if !ok || description == "" {
			t.Errorf("%q is not reserved with a description", name)
		}
	}
}
