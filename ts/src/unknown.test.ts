// SPDX-License-Identifier: Apache-2.0

import assert from 'node:assert/strict';
import { test } from 'node:test';

import { version } from './buildinfo.ts';
import { buildFields } from './logging.ts';

test('a service that never sets its version reports unknown', () => {
  assert.equal(version(), 'unknown');
});

test('log build fields use the shared label names', () => {
  assert.deepEqual(Object.keys(buildFields()).sort(), ['build_version', 'instrumentation_version']);
  assert.equal(buildFields().build_version, 'unknown');
});
