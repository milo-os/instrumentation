// SPDX-License-Identifier: Apache-2.0

import assert from 'node:assert/strict';
import { test } from 'node:test';

import { isReserved, Project, reservedLabels, ResourceGroup } from './labels.ts';

test('generated constants carry the shared names', () => {
  assert.equal(Project, 'project');
  assert.equal(ResourceGroup, 'resource_group');
});

test('reserved labels are reported with their reason, and others are not', () => {
  assert.match(isReserved('project_name') ?? '', /./);
  assert.equal(isReserved('project'), undefined);
  assert.equal(isReserved('toString'), undefined);
  assert.ok(reservedLabels().includes('namespace'));
});
