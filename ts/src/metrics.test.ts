// SPDX-License-Identifier: Apache-2.0

import assert from 'node:assert/strict';
import { test } from 'node:test';

import { Counter, register } from '@prometheus-io/client';

import { Outcome, ResourceKind } from './labels.ts';
import { BuildInfoName, newCounter, newGauge, newHistogram, newSummary } from './metrics.ts';

test('importing the module registers the build info metric', async () => {
  assert.match(await register.metrics(), new RegExp(`^${BuildInfoName}\\{build_version="[^"]+",instrumentation_version="ts/v[^"]+"\\} 1$`, 'm'));
});

test('a metric lands on the registry the service serves', async () => {
  newCounter({ name: 'instrumentation_test_served_total', help: 'Served.', labelNames: [Outcome] }).inc({ [Outcome]: 'success' });
  assert.match(await register.metrics(), /^instrumentation_test_served_total\{outcome="success"\} 1$/m);
});

test('a reserved label is refused, naming the label and the fix', () => {
  assert.throws(
    () => newCounter({ name: 'instrumentation_test_reserved_total', help: 'Reserved.', labelNames: ['namespace'] }),
    /label "namespace" is reserved for the collection pipeline .* @miloapis\/instrumentation\/labels/,
  );
  assert.equal(register.getSingleMetric('instrumentation_test_reserved_total'), undefined);
});

test('a build label is refused', () => {
  assert.throws(
    () => newGauge({ name: 'instrumentation_test_build', help: 'Build.', labelNames: ['build_version'] }),
    /would replace every series on each deploy; join on instrumentation_build_info/,
  );
});

test('creating a metric again with the same definition returns it', () => {
  const config = { name: 'instrumentation_test_again_total', help: 'Again.', labelNames: [ResourceKind] };
  assert.equal(newCounter(config), newCounter(config));
});

test('a different definition for the same name is refused, naming both', () => {
  newHistogram({ name: 'instrumentation_test_shape_seconds', help: 'Shape.', buckets: [0.1, 1] });
  assert.throws(
    () => newHistogram({ name: 'instrumentation_test_shape_seconds', help: 'Shape.', buckets: [0.5] }),
    /already created as histogram with labels \[\] buckets \[0\.1,1\], now requested as histogram with labels \[\] buckets \[0\.5\]/,
  );
  assert.throws(() => newGauge({ name: 'instrumentation_test_shape_seconds', help: 'Shape.' }), /already created as histogram/);
  newSummary({ name: 'instrumentation_test_summary', help: 'Summary.', labelNames: [Outcome] });
  assert.throws(() => newSummary({ name: 'instrumentation_test_summary', help: 'Summary.', labelNames: [ResourceKind] }), /labels \[outcome\].*labels \[resource_kind\]/);
});

test('a metric with a collect function can be created only once', () => {
  const config = { name: 'instrumentation_test_collect', help: 'Collect.', collect() {} };
  newGauge(config);
  assert.throws(() => newGauge(config), /can be created only once/);
});

test('a metric registered outside the library is refused', () => {
  new Counter({ name: 'instrumentation_test_outside_total', help: 'Outside.' });
  assert.throws(() => newCounter({ name: 'instrumentation_test_outside_total', help: 'Outside.' }), /already registered outside/);
});

test('clearing the registry lets metrics and the build info come back', async () => {
  register.clear();
  newCounter({ name: 'instrumentation_test_after_clear_total', help: 'After clear.' }).inc();
  const body = await register.metrics();
  assert.match(body, /^instrumentation_test_after_clear_total 1$/m);
  assert.match(body, new RegExp(`^${BuildInfoName}\\{`, 'm'));
});
