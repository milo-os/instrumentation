import type { Registry } from '@prometheus-io/client';
import { newCounter, observeRequest, scrape } from '@miloapis/instrumentation/metrics';
