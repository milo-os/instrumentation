# @miloapis/instrumentation

Uniform logging and metrics for Milo services written in TypeScript. The Go
module, `go.miloapis.com/instrumentation`, shares its label names and build
info metric.

## Install

```sh
npm install @miloapis/instrumentation @prometheus-io/client
```

Requires Node.js 24 or later, or Bun, and `@prometheus-io/client` 0.16, the
package `prom-client` was renamed to.

## Migrate from prom-client

Every metric must live on the registry this package serves, so adopting it
means moving off `prom-client` entirely:

1. Replace `prom-client` imports with `@prometheus-io/client`. Gauges no
   longer start at zero, so set one before relying on its value.
2. Replace `@hono/prometheus` with `scrape()` and `observeRequest()`, below.
   The HTTP metrics keep their names, labels and buckets, so dashboards
   don't change.

## Create metrics

```ts
import { Outcome, ResourceKind } from '@miloapis/instrumentation/labels';
import { newCounter } from '@miloapis/instrumentation/metrics';

const mutations = newCounter({
  name: 'portal_resource_mutations_total',
  help: 'Resource mutations, by kind and outcome.',
  labelNames: [ResourceKind, Outcome],
});
```

Constructors cover counters, gauges, histograms and summaries, and register
on `@prometheus-io/client`'s default registry.

Creating the same metric again with an identical definition returns the
existing one, so a module may load more than once. Any difference in type,
label names, buckets or percentiles throws, as does creating a metric with a
`collect` function twice. A label reserved for the collection pipeline, such
as `namespace` or `cluster`, throws.

## Serve metrics

Serve `scrape()` at `/metrics`, and call `observeRequest()` from your
request middleware. With Hono:

```ts
import { observeRequest, scrape } from '@miloapis/instrumentation/metrics';

app.use('*', async (c, next) => {
  const start = performance.now();
  try {
    await next();
  } finally {
    observeRequest({
      method: c.req.method,
      route: c.req.routePath,
      status: c.res.status,
      seconds: (performance.now() - start) / 1000,
    });
  }
});

app.get('/metrics', async (c) => {
  const { body, contentType } = await scrape();
  return c.text(body, 200, { 'Content-Type': contentType });
});
```

`observeRequest` records `http_requests_total` and
`http_request_duration_seconds`, labelled `method`, `status`, `ok` and
`route`. Pass the matched route pattern, never the raw path.

## Report the build version

Call `setVersion` at startup, before anything reads the version:

```ts
import { setVersion } from '@miloapis/instrumentation/buildinfo';

setVersion(process.env.APP_VERSION ?? '');
```

Importing the metrics module registers `instrumentation_build_info`, labelled
with `build_version` and `instrumentation_version`. This package reports its
own version prefixed like its release tag, such as `ts/v0.1.0`, so dashboards
tell it apart from the Go module.

## Log

This package doesn't replace your logger. Merge its build fields into the
context written on every line:

```ts
import { buildFields } from '@miloapis/instrumentation/logging';

const logger = new Logger({ ...buildFields(), requestId });
```

This library does not redact. Don't log secrets.

## Check adoption

`assertAdopted()` from `@miloapis/instrumentation/test` fails if any source
imports `prom-client` or `@hono/prometheus`, builds metrics directly on
`@prometheus-io/client`, or never serves `scrape()`. It needs `typescript`
installed. See
[ADOPTING.md](https://github.com/milo-os/instrumentation/blob/main/ADOPTING.md)
for the test to add and how to fix what it reports.

## Labels

Label constants are generated from
[labels.yaml](https://github.com/milo-os/instrumentation/blob/main/labels.yaml),
shared with every language. `isReserved(name)` reports why a name is reserved
for the collection pipeline.

## License

[Apache 2.0](LICENSE)
