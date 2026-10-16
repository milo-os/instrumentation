// SPDX-License-Identifier: Apache-2.0

import assert from 'node:assert/strict';
import { test } from 'node:test';

import { Project } from './labels.ts';
import { BuildInfoName, newCounter, observeRequest, scrape } from './metrics.ts';

test('scrape serves the library metrics and the build info', async () => {
  newCounter({ name: 'instrumentation_test_scraped_total', help: 'Scraped.', labelNames: [Project] }).inc({ [Project]: 'p1' });
  const { body, contentType } = await scrape();
  assert.match(contentType, /^text\/plain/);
  assert.match(body, /^instrumentation_test_scraped_total\{project="p1"\} 1$/m);
  assert.match(body, new RegExp(`^${BuildInfoName}\\{`, 'm'));
});

test('observeRequest records what @hono/prometheus recorded', async () => {
  observeRequest({ method: 'GET', route: '/orgs/:org', status: 200, seconds: 0.03 });
  observeRequest({ method: 'POST', route: '/orgs/:org', status: 503, seconds: 2 });
  observeRequest({ method: 'GET', route: '/orgs/:org', status: 200, seconds: 0.03 });
  const { body } = await scrape();
  assert.match(body, /^# HELP http_requests_total Total number of HTTP requests$/m);
  assert.match(body, /^http_requests_total\{method="GET",status="200",ok="true",route="\/orgs\/:org"\} 2$/m);
  assert.match(body, /^http_requests_total\{method="POST",status="503",ok="false",route="\/orgs\/:org"\} 1$/m);
  assert.match(body, /^# HELP http_request_duration_seconds Duration of HTTP requests in seconds$/m);
  assert.match(body, /^http_request_duration_seconds_bucket\{le="0\.05",method="GET",status="200",ok="true",route="\/orgs\/:org"\} 2$/m);
  assert.match(body, /^http_request_duration_seconds_bucket\{le="2\.5",method="POST",status="503",ok="false",route="\/orgs\/:org"\} 1$/m);
  assert.match(body, /^http_request_duration_seconds_bucket\{le="0\.005",/m);
  assert.match(body, /^http_request_duration_seconds_bucket\{le="10",/m);
});
