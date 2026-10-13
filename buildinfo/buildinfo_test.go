// SPDX-License-Identifier: Apache-2.0

package buildinfo

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestResolve(t *testing.T) {
	stamped := func(main string) func() (*debug.BuildInfo, bool) {
		return func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{Main: debug.Module{Version: main}}, true
		}
	}

	for _, tc := range []struct {
		name     string
		override string
		linked   string
		read     func() (*debug.BuildInfo, bool)
		want     string
	}{
		{"override wins", "v1.0.0", "v2.0.0", stamped("v3.0.0"), "v1.0.0"},
		{"linker flag next", "", "v2.0.0", stamped("v3.0.0"), "v2.0.0"},
		{"module version next", "", "", stamped("v3.0.0"), "v3.0.0"},
		{"pseudo-version from a modified checkout", "", "", stamped("v0.0.0-20261013214129-359e97b303f7+dirty"), "v0.0.0-20261013214129-359e97b303f7+dirty"},
		{"docker build without vcs", "", "", stamped("(devel)"), Unknown},
		{"no build info", "", "", func() (*debug.BuildInfo, bool) { return nil, false }, Unknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolve(tc.override, tc.linked, tc.read); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestVersionIsFrozenOnFirstRead(t *testing.T) {
	t.Cleanup(reset)
	reset()
	SetVersion("v1.2.3")
	if got := Version(); got != "v1.2.3" {
		t.Fatalf("Version() = %q, want v1.2.3", got)
	}

	SetVersion("v1.2.3")

	defer func() {
		msg, _ := recover().(string)
		if !strings.Contains(msg, "already reported") || !strings.Contains(msg, "TestVersionIsFrozenOnFirstRead") {
			t.Errorf("SetVersion after a read did not panic naming the first reader: %q", msg)
		}
	}()
	SetVersion("v9.9.9")
}

func TestInstrumentationVersion(t *testing.T) {
	for _, tc := range []struct {
		name string
		info debug.BuildInfo
		want string
	}{
		{
			name: "dependency",
			info: debug.BuildInfo{Main: debug.Module{Path: "go.datum.net/nso", Version: "v1.4.2"}, Deps: []*debug.Module{{Path: modulePath, Version: "v0.1.0"}}},
			want: "v0.1.0",
		},
		{
			name: "replaced dependency",
			info: debug.BuildInfo{Main: debug.Module{Path: "go.datum.net/nso"}, Deps: []*debug.Module{{Path: modulePath, Version: "v0.1.0", Replace: &debug.Module{Path: "../instrumentation", Version: "(devel)"}}}},
			want: Unknown,
		},
		{
			name: "this module",
			info: debug.BuildInfo{Main: debug.Module{Path: modulePath, Version: "(devel)"}},
			want: Unknown,
		},
		{
			name: "not linked",
			info: debug.BuildInfo{Main: debug.Module{Path: "go.datum.net/nso", Version: "v1.4.2"}},
			want: Unknown,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := instrumentationVersion(&tc.info); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
