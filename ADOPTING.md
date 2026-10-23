# Adopting instrumentation

This guide is for engineers and coding agents adding
`go.miloapis.com/instrumentation` or `@miloapis/instrumentation` to a service,
or fixing what its adoption check reports. Every check failure links here.

Adopting means three things in either language:

1. Every metric is created through the library, so reserved labels are
   refused and the metric is served.
1. The service serves the library's registry at `/metrics`.
1. One test, the adoption check, fails the build if either stops being true.

## Go

### Adopt the Go module

1. Add the module. It requires Go 1.26 or later:

   ```sh
   go get go.miloapis.com/instrumentation
   ```

   `go.mod` records the exact version this resolves; upgrade deliberately.

1. Replace every `promauto.New*`, `prometheus.MustRegister`, and
   `legacyregistry.MustRegister` with the constructors in
   `go.miloapis.com/instrumentation/metrics`, such as `metrics.NewCounterVec`.
   Name labels with the constants in `go.miloapis.com/instrumentation/labels`.
1. In `main`, before anything logs, set the version and the logger:

   ```go
   buildinfo.SetVersion(version)
   ctrl.SetLogger(logging.New(&opts))
   ```

   Don't set `Development: true` in the zap options; pass `--zap-devel` for
   local runs instead.
1. If `/metrics` isn't served by a controller-runtime manager, as in a
   k8s.io/apiserver, serve `metrics.Handler()` there, adding the server's own
   registry with `metrics.WithGatherers`.

### Add the Go adoption check

Add this test to any package in the module, for example as
`instrumentation_test.go`:

```go
package main

import (
    "testing"

    "go.miloapis.com/instrumentation/instrumenttest"
)

func TestInstrumentationAdopted(t *testing.T) {
    instrumenttest.AssertAdopted(t)
}
```

It reads the module's non-test source, so it works wherever it lives. To leave
out a directory, such as generated code, pass
`instrumenttest.SkipDir("path/from/module/root")`.

### Fix what the Go check reports

| The check reports | Fix |
|---|---|
| Uses Prometheus's default registry | Create the metric with `go.miloapis.com/instrumentation/metrics` |
| Registers metrics outside the library | Use the library's constructors, or `metrics.MustRegister` for a custom collector |
| Serves component-base's registry | Serve `metrics.Handler(metrics.WithGatherers(legacyregistry.DefaultGatherer))` instead of `legacyregistry.Handler()` |
| The manager's metrics server is disabled | Serve `metrics.Handler()` on the server that answers `/metrics` |
| Builds a logger without the standard defaults | Build it with `logging.New` or `logging.NewRaw` |
| Turns on development mode | Remove `Development: true`; pass `--zap-devel` locally |
| No file calls SetLogger with logging.New | Call `ctrl.SetLogger(logging.New(&opts))` in `main` |
| A label is reserved for the collection pipeline | Rename it with a constant from the `labels` package, and update every `prometheus.Labels` map that names it |

## TypeScript

### Adopt the TypeScript package

1. Add the package. It requires Node.js 24 or later, or Bun:

   ```sh
   npm install --save-exact @miloapis/instrumentation @prometheus-io/client
   ```

   `--save-exact` records the exact versions installed rather than a range,
   so the service changes versions only when someone upgrades it.

1. Remove `prom-client` and `@hono/prometheus`. Create every metric with
   `newCounter`, `newGauge`, `newHistogram`, or `newSummary` from
   `@miloapis/instrumentation/metrics`, naming labels with the constants in
   `@miloapis/instrumentation/labels`.
1. Serve `scrape()` at `/metrics`, and call `observeRequest()` from request
   middleware. The package README shows both with Hono.
1. Call `setVersion` at startup, and merge `buildFields()` into your logger's
   context.

### Add the TypeScript adoption check

Add `typescript` as a development dependency if the service doesn't have it:

```sh
npm install --save-dev --save-exact typescript
```

Then add this test, for example as `src/instrumentation.test.ts`:

```ts
import { test } from 'node:test';

import { assertAdopted } from '@miloapis/instrumentation/test';

test('instrumentation is adopted', () => {
  assertAdopted();
});
```

Under Bun, import `test` from `bun:test` instead. The check scans the current
directory's source, leaving out tests, declaration files, `node_modules`,
`dist`, `build`, and `coverage`. To leave out another directory, pass
`assertAdopted({ skip: ['src/generated'] })`.

### Fix what the TypeScript check reports

| The check reports | Fix |
|---|---|
| Imports prom-client | Import from `@miloapis/instrumentation/metrics`, and `@prometheus-io/client` for types only |
| Imports @hono/prometheus | Serve `scrape()` and call `observeRequest()` from your middleware |
| Imports @prometheus-io/client directly | Create metrics with the library's constructors. `import type` is allowed. |
| Nothing imports scrape() | Serve `scrape()` at `/metrics` |
| A label is reserved for the collection pipeline | Rename it with a constant from `@miloapis/instrumentation/labels` |

## Tell agents in your repository

Add this section to the service's `AGENTS.md`, or `CLAUDE.md`, so coding
agents know the rule before the check fails:

```markdown
## Metrics and logs

Create every metric and logger through go.miloapis.com/instrumentation (Go)
or @miloapis/instrumentation (TypeScript), naming labels with its labels
constants. The instrumentation adoption test enforces this. When it fails,
follow its message, which links to
https://github.com/milo-os/instrumentation/blob/main/ADOPTING.md.
```
