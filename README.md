# instrumentation

Uniform logging and metrics for Milo services.

## Install

```sh
go get go.miloapis.com/instrumentation
```

Requires Go 1.25 or later and controller-runtime v0.20.4 or later.

## Find unexported metrics and unstandard loggers

Add these tests to any package in your module:

```go
func TestMetricsAreExported(t *testing.T) {
    instrumenttest.AssertNothingStranded(t)
}

func TestStandardLogger(t *testing.T) {
    instrumenttest.AssertStandardLogger(t)
}
```

Both read your module's non-test source, so they work wherever the test
lives.

`AssertNothingStranded` fails at:

- every use of `prometheus.DefaultRegisterer`: `promauto.New*`,
  `prometheus.MustRegister`, `prometheus.Register`, and `promhttp.Handler`.
  Neither a controller-runtime manager nor a k8s.io/apiserver serves that
  registry. Passing `prometheus.DefaultGatherer` to `metrics.WithGatherers` is
  allowed;
- every direct use of controller-runtime's registry, `promauto.With`, or
  registration on component-base's `legacyregistry`, which skips the
  reserved-label check and `Lint`;
- `legacyregistry.Handler`, which serves none of the metrics created through
  this library;
- a manager whose metrics server is disabled with `BindAddress: "0"`, when
  nothing in the module serves metrics another way.

It cannot see registrations made inside your dependencies, and it only
recognizes `BindAddress: "0"` written as a string literal. An address from a
variable or constant counts as neither serving nor disabled, so a module that
sets its address from configuration must also serve `metrics.Handler`.

`AssertStandardLogger` checks the module as a whole: a binary that never sets
a logger passes if another file in the module does.

`AssertStandardLogger` fails where code builds a logger another way, such as
`zap.New`, `slog.New`, or `funcr.New`, and at every `SetLogger` call in a
file that never calls `logging.New`. It also fails if no file both calls
`SetLogger` and builds the logger with `logging.New`, because until then
controller-runtime drops its log lines and klog writes plain text.

To exempt a directory, pass `instrumenttest.SkipDir("path/from/module/root")`.
A path that does not exist fails the test. Files with `//go:build ignore` are
skipped.

## Create metrics

```go
var programmed = metrics.NewGaugeVec(prometheus.GaugeOpts{
    Namespace: "nso",
    Subsystem: "gateway",
    Name:      "programmed",
    Help:      "1 if the Gateway is Programmed at its current generation.",
}, []string{labels.Project, labels.ResourceNamespace, labels.ResourceName})
```

Every constructor registers on the registry a controller-runtime manager
serves. Constructors cover counters, gauges, histograms, and summaries, their
vectors, and gauge and counter functions. Creating the same metric again with
an identical definition returns the existing one. A metric name has one
definition: any difference in type, label order, buckets, or objectives
panics, including between variants with different constant-label values, as
does creating a function metric twice. A label reserved for the collection
pipeline, such as `cluster` or `namespace`, panics at startup.

When you rename a reserved label, also update every place that builds labels
by name, such as `prometheus.Labels` maps passed to `DeletePartialMatch` or
`With`. A map with the old key matches nothing and no check catches it, so
old series are never cleared. Use the `labels` constants there too. For a collector that computes
values at scrape time, use `metrics.MustRegister`.

A binary whose metrics endpoint is not a manager's, including one that sets
the manager's `BindAddress` to `"0"`, serves the same registry with
`metrics.Handler()`.

### Serve other registries

Add the registries a binary already serves with `metrics.WithGatherers`. A
k8s.io/apiserver turns off its own `/metrics` with `EnableMetrics = false` and
serves both registries on the same path:

```go
mux.Handle("/metrics", metrics.Handler(
    metrics.WithGatherers(legacyregistry.DefaultGatherer),
))
```

Pass `prometheus.DefaultGatherer` too if a dependency registers there.

When another registry already exports the Go runtime and process metrics,
controller-runtime's copies are dropped so the two don't collide. Any other
series exported by two registries fails the whole scrape, so rename one of
them. Catch that in a test that builds the handler the way the binary does
and links the same packages:

```go
func TestMetricsScrape(t *testing.T) {
    instrumenttest.AssertScrapes(t, server.MetricsHandler())
}
```

Lint metric names in a package that imports every package defining metrics:

```go
func TestMetricsLint(t *testing.T) {
    instrumenttest.Lint(t)
}
```

`Lint` briefly adds a placeholder series to each metric vector, so don't run
it in parallel with tests that count series.

## Log

```go
func main() {
    buildinfo.SetVersion(version)

    opts := zap.Options{}
    opts.BindFlags(flag.CommandLine)
    flag.Parse()
    ctrl.SetLogger(logging.New(&opts))
}
```

Production logs are JSON at info level with no sampling, with UTC timestamps
to the nanosecond.

This library does not redact. Don't log secrets.

client-go's own lines, such as leader election, go through klog. To give them
the same format, also pass the logger to `klog.SetLogger`.

Code that takes a `*slog.Logger` can wrap the same logger:
`slog.New(logr.ToSlogHandler(logger))`.

Use the same `labels` constants as log keys, so log lines join with metrics
about the same resource:

```go
log.FromContext(ctx).Info("programmed",
    labels.Project, project,
    labels.ResourceName, name)
```

## Versions

Every log line carries `build_version` and `instrumentation_version`, and
importing `metrics` registers `instrumentation_build_info` with the same two
labels. To break a query down by version, join on that metric rather than
adding the version to every series:

```promql
sum by (build_version) (
  rate(nso_reconcile_errors_total[5m])
  * on (cluster, namespace, pod) group_left (build_version)
  instrumentation_build_info
)
```

`instrumentation_version` comes from the binary's build information. The
binary's `build_version` comes from, in order:

1. `buildinfo.SetVersion`.
2. `-ldflags "-X go.miloapis.com/instrumentation/buildinfo.version=v1.2.3"`.
3. The module version Go stamps at build time: the tag, or a pseudo-version
   such as `v0.0.0-20261013214129-359e97b303f7`, with `+dirty` for a
   modified tree.

Docker builds that copy source without `.git` get no stamp, so set the
version with 1 or 2. The version is fixed the first time it is read, so call
`buildinfo.SetVersion` first thing in `main`.

## Labels

[labels.yaml](labels.yaml) is the only list of label names, for metric labels
and log keys alike. After editing it, run `task generate`.

| Group | Set by | Examples |
|---|---|---|
| Subject | Your code, as constants in `labels` | `organization`, `project`, `resource_name`, `reason` |
| Build | This library | `build_version`, `instrumentation_version` |
| Reserved | The collection pipelines | `app`, `cluster`, `namespace`, `pod`, `project_name` |

Code that sets a reserved label collides with the pipeline's value. Metric
constructors refuse reserved names; log keys are not checked. Never use
`project_name` as a log key: the customer telemetry pipeline uses it to route
a record into that project's tenant.

## Development

Run `task ci` before you push. GitHub CI runs the same command, so a change
that passes locally passes in CI.

## License

Apache License 2.0. See [LICENSE](LICENSE).
