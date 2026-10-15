// SPDX-License-Identifier: Apache-2.0

// Package buildinfo reports the version of the running binary and of this
// library, for the build_version and instrumentation_version labels.
//
// The binary's version comes from, in order:
//   - SetVersion.
//   - The linker flag -X go.miloapis.com/instrumentation/buildinfo.version=<v>.
//   - The module version Go stamps at build time, unless it is (devel). From
//     a git checkout that is the tag, or a pseudo-version such as
//     v0.0.0-20261013214129-359e97b303f7, with +dirty for a modified tree.
//
// Docker builds that copy source without .git get no stamp, so set the
// version explicitly there.
//
// The version is fixed the first time it is read, so logs and metrics always
// agree. Call SetVersion first thing in main.
package buildinfo

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
)

const modulePath = "go.miloapis.com/instrumentation"

// Unknown is reported when no version is available.
const Unknown = "unknown"

const devel = "(devel)"

var version string

var (
	mu       sync.Mutex
	override string
	frozen   string
	isFrozen bool
	readBy   string
)

// SetVersion sets the binary's version. It panics if the version has already
// been read, because something would already be reporting a different value.
func SetVersion(v string) {
	mu.Lock()
	defer mu.Unlock()
	if isFrozen && v != frozen {
		panic(fmt.Sprintf("buildinfo.SetVersion(%q) called after version %q was already reported; call it before logging.New and before serving metrics. First read:\n%s", v, frozen, readBy))
	}
	override = v
}

// Version returns the binary's version.
func Version() string {
	mu.Lock()
	defer mu.Unlock()
	if !isFrozen {
		frozen = resolve(override, version, readBuildInfo)
		isFrozen = true
		readBy = callers()
	}
	return frozen
}

func callers() string {
	pcs := make([]uintptr, 16)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(3, pcs)])
	var b strings.Builder
	for {
		f, more := frames.Next()
		fmt.Fprintf(&b, "\t%s\n\t\t%s:%d\n", f.Function, f.File, f.Line)
		if !more {
			return b.String()
		}
	}
}

// InstrumentationVersion returns the version of this library linked into the
// binary.
func InstrumentationVersion() string {
	return instrumentationVersionOnce()
}

var instrumentationVersionOnce = sync.OnceValue(func() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return Unknown
	}
	return instrumentationVersion(info)
})

func readBuildInfo() (*debug.BuildInfo, bool) { return debug.ReadBuildInfo() }

func resolve(override, linked string, read func() (*debug.BuildInfo, bool)) string {
	if override != "" {
		return override
	}
	if linked != "" {
		return linked
	}
	info, ok := read()
	if !ok {
		return Unknown
	}
	return known(info.Main.Version)
}

func instrumentationVersion(info *debug.BuildInfo) string {
	if info.Main.Path == modulePath {
		return known(info.Main.Version)
	}
	for _, dep := range info.Deps {
		if dep.Path != modulePath {
			continue
		}
		if dep.Replace != nil {
			return known(dep.Replace.Version)
		}
		return known(dep.Version)
	}
	return Unknown
}

func known(v string) string {
	if v == "" || v == devel {
		return Unknown
	}
	return v
}

func reset() {
	mu.Lock()
	defer mu.Unlock()
	override, frozen, isFrozen, readBy = "", "", false, ""
}
