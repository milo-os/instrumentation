// SPDX-License-Identifier: Apache-2.0

package instrumenttest

import "testing"

// guide explains how to fix everything the checks in this package report.
const guide = "https://github.com/milo-os/instrumentation/blob/main/ADOPTING.md"

const seeGuide = "; see " + guide

// AssertAdopted fails unless the current module both exports every metric
// and logs through the standard logger. It runs AssertNothingStranded and
// AssertStandardLogger with the same options, and is the one check a service
// needs.
func AssertAdopted(t testing.TB, opts ...Option) {
	t.Helper()
	assertAdopted(t, currentModuleRoot(t), newConfig(opts))
}

func assertAdopted(t testing.TB, root string, cfg config) {
	t.Helper()
	assertNothingStranded(t, root, cfg)
	assertStandardLogger(t, root, cfg)
}
