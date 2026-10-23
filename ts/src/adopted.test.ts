// SPDX-License-Identifier: Apache-2.0

import assert from 'node:assert/strict';
import { test } from 'node:test';

import { assertAdopted } from './test.ts';

const fixture = (name: string) => new URL(`../testdata/adopted/${name}`, import.meta.url).pathname;

const findings = (root: string, skip?: string[]): string[] => {
  try {
    assertAdopted({ root, skip });
  } catch (e) {
    return (e as Error).message.split('\n');
  }
  return [];
};

test('every bypass is reported with its line, and type-only imports are not', () => {
  const lines = findings(fixture('bad'));
  const where = lines.map((l) => l.slice(0, l.indexOf(': ')));
  assert.deepEqual(where, [
    'src/server.ts:1',
    'src/server.ts:2',
    'src/server.ts:3',
    'src/server.ts:4',
    'src/server.ts:7',
    'src/server.ts:8',
    'src/server.ts:9',
  ]);
  assert.match(lines[0] ?? '', /imports prom-client, which was renamed @prometheus-io\/client/);
  assert.match(lines[1] ?? '', /imports @hono\/prometheus/);
  assert.match(lines[2] ?? '', /imports @prometheus-io\/client directly/);
});

test('every finding links the guide', () => {
  for (const line of findings(fixture('bad'))) {
    assert.ok(line.endsWith('; see https://github.com/milo-os/instrumentation/blob/main/ADOPTING.md'), line);
  }
});

test('an adopted service passes, including a renamed scrape import', () => {
  assert.deepEqual(findings(fixture('good')), []);
  assert.deepEqual(findings(fixture('aliased')), []);
});

test('metrics nothing serves are reported', () => {
  const lines = findings(fixture('unserved'));
  assert.equal(lines.length, 1);
  assert.match(lines[0] ?? '', /nothing under .* imports scrape\(\)/);
});

test('a skipped directory is left out, and a missing one is refused', () => {
  assert.equal(findings(fixture('skipped')).length, 1);
  assert.match(findings(fixture('skipped'))[0] ?? '', /^generated\/client\.ts:1: imports prom-client/);
  assert.deepEqual(findings(fixture('skipped'), ['generated']), []);
  assert.throws(() => assertAdopted({ root: fixture('good'), skip: ['nope'] }), /skip "nope" does not exist/);
});

test('a directory with no source is refused rather than passed', () => {
  assert.throws(() => assertAdopted({ root: fixture('empty') }), /no source found/);
});
