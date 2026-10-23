import { register } from 'prom-client';
import { prometheus } from '@hono/prometheus';
import { Counter } from '@prometheus-io/client';
import * as client from '@prometheus-io/client';
import { scrape } from '@miloapis/instrumentation/metrics';

const legacy = require('prom-client');
export { Gauge } from '@prometheus-io/client';
const later = await import('@hono/prometheus');
