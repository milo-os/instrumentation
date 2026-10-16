// SPDX-License-Identifier: Apache-2.0

import assert from 'node:assert/strict';
import { test } from 'node:test';
import { readFileSync } from 'node:fs';

import { instrumentationVersion, setVersion, version } from './buildinfo.ts';

test('the version is the one set before the first read, and then frozen', () => {
  setVersion('v1.2.3');
  assert.equal(version(), 'v1.2.3');
  setVersion('v1.2.3');
  assert.throws(() => setVersion('v9.9.9'), /already read as "v1\.2\.3"; call setVersion before anything reads it/);
  assert.equal(version(), 'v1.2.3');
});

test('the library version comes from package.json, prefixed like its tag', () => {
  const pkg = JSON.parse(readFileSync(new URL('../package.json', import.meta.url), 'utf8')) as { version: string };
  assert.equal(instrumentationVersion(), `ts/v${pkg.version}`);
});
