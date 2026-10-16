// SPDX-License-Identifier: Apache-2.0

/**
 * Creates Prometheus metrics on @prometheus-io/client's default registry, and
 * serves it. Serve scrape() at /metrics; nothing else registered elsewhere is
 * exported.
 *
 * Use the constants in ./labels for label names. Constructors throw if a
 * label name is reserved for the collection pipeline or is a build label.
 * Creating a metric again with an identical definition returns the existing
 * one, so a module may be loaded more than once. A metric name has one
 * definition: any difference in type, label names, buckets, or percentiles
 * throws, as does creating a metric with a collect function a second time.
 *
 * Importing this module registers instrumentation_build_info, which carries
 * the build_version and instrumentation_version labels.
 */

import {
  Counter,
  Gauge,
  Histogram,
  register,
  Summary,
  type CounterConfiguration,
  type GaugeConfiguration,
  type HistogramConfiguration,
  type Metric,
  type SummaryConfiguration,
} from '@prometheus-io/client';

import { instrumentationVersion, version } from './buildinfo.ts';
import { BuildVersion, InstrumentationVersion } from './labels.generated.ts';
import { isReserved } from './labels.ts';

/** The name of the metric that carries the build labels. */
export const BuildInfoName = 'instrumentation_build_info';

type Kind = 'counter' | 'gauge' | 'histogram' | 'summary';

interface Definition {
  kind: Kind;
  labelNames: string;
  shape: string;
}

const definitions = new Map<string, Definition>();

const describe = (d: Definition) => `${d.kind} with labels [${d.labelNames}]${d.shape ? ` ${d.shape}` : ''}`;

function checkLabel(metric: string, label: string): void {
  if (label === BuildVersion || label === InstrumentationVersion) {
    throw new Error(`metric ${metric}: label ${JSON.stringify(label)} would replace every series on each deploy; join on ${BuildInfoName} instead`);
  }
  const why = isReserved(label);
  if (why !== undefined) {
    throw new Error(
      `metric ${metric}: label ${JSON.stringify(label)} is reserved for the collection pipeline (${why}); rename it with a constant from @miloapis/instrumentation/labels`,
    );
  }
}

function create<M extends Metric>(
  kind: Kind,
  name: string,
  labelNames: readonly string[] | undefined,
  shape: string,
  hasCollect: boolean,
  make: () => M,
): M {
  for (const label of labelNames ?? []) {
    checkLabel(name, label);
  }
  ensureBuildInfo();
  const definition: Definition = { kind, labelNames: (labelNames ?? []).join(','), shape };
  const existing = register.getSingleMetric(name);
  if (existing === undefined) {
    const metric = make();
    definitions.set(name, definition);
    return metric;
  }
  const recorded = definitions.get(name);
  if (recorded === undefined) {
    throw new Error(`metric ${name}: already registered outside @miloapis/instrumentation/metrics`);
  }
  if (describe(recorded) !== describe(definition)) {
    throw new Error(`metric ${name}: already created as ${describe(recorded)}, now requested as ${describe(definition)}`);
  }
  if (hasCollect) {
    throw new Error(`metric ${name}: a metric with a collect function can be created only once; keep the first one`);
  }
  return existing as M;
}

/** Creates and registers a counter. */
export function newCounter<T extends string = string>(config: Omit<CounterConfiguration<T>, 'registers'>): Counter<T> {
  return create('counter', config.name, config.labelNames, '', config.collect !== undefined, () => new Counter<T>({ ...config, registers: [register] }));
}

/** Creates and registers a gauge. */
export function newGauge<T extends string = string>(config: Omit<GaugeConfiguration<T>, 'registers'>): Gauge<T> {
  return create('gauge', config.name, config.labelNames, '', config.collect !== undefined, () => new Gauge<T>({ ...config, registers: [register] }));
}

/** Creates and registers a histogram. */
export function newHistogram<T extends string = string>(config: Omit<HistogramConfiguration<T>, 'registers'>): Histogram<T> {
  const shape = `buckets ${config.buckets ? JSON.stringify(config.buckets) : 'default'}`;
  return create('histogram', config.name, config.labelNames, shape, config.collect !== undefined, () => new Histogram<T>({ ...config, registers: [register] }));
}

/** Creates and registers a summary. */
export function newSummary<T extends string = string>(config: Omit<SummaryConfiguration<T>, 'registers'>): Summary<T> {
  const shape = `percentiles ${config.percentiles ? JSON.stringify(config.percentiles) : 'default'} window ${config.maxAgeSeconds ?? 'none'}/${config.ageBuckets ?? 'none'}`;
  return create('summary', config.name, config.labelNames, shape, config.collect !== undefined, () => new Summary<T>({ ...config, registers: [register] }));
}

/** The metrics exposition to serve at /metrics, with its content type. */
export async function scrape(): Promise<{ body: string; contentType: string }> {
  ensureBuildInfo();
  return { body: await register.metrics(), contentType: register.contentType };
}

const httpLabels = ['method', 'status', 'ok', 'route'] as const;

/** One handled HTTP request, for observeRequest. */
export interface Request {
  /** The request method, such as GET. */
  method: string;
  /** The matched route pattern, such as /orgs/:org. Never the raw path. */
  route: string;
  /** The response status code. */
  status: number;
  /** How long the request took, in seconds. */
  seconds: number;
}

/**
 * Records a handled HTTP request on http_requests_total and
 * http_request_duration_seconds, with the names, labels and buckets
 * @hono/prometheus uses. Call it from your framework's middleware.
 */
export function observeRequest(request: Request): void {
  const labels = {
    method: request.method,
    status: String(request.status),
    ok: String(request.status >= 200 && request.status < 300),
    route: request.route,
  };
  newCounter({ name: 'http_requests_total', help: 'Total number of HTTP requests', labelNames: httpLabels }).inc(labels);
  newHistogram({
    name: 'http_request_duration_seconds',
    help: 'Duration of HTTP requests in seconds',
    labelNames: httpLabels,
    buckets: [0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10],
  }).observe(labels, request.seconds);
}

function ensureBuildInfo(): void {
  if (register.getSingleMetric(BuildInfoName) !== undefined) {
    return;
  }
  new Gauge({
    name: BuildInfoName,
    help: 'Always 1. Labelled with the version of the running service and of go.miloapis.com/instrumentation or @miloapis/instrumentation.',
    labelNames: [BuildVersion, InstrumentationVersion],
    registers: [register],
    collect() {
      this.reset();
      this.set({ [BuildVersion]: version(), [InstrumentationVersion]: instrumentationVersion() }, 1);
    },
  });
}

ensureBuildInfo();
