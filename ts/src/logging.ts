// SPDX-License-Identifier: Apache-2.0

/**
 * Fields every log line carries. This package does not replace your logger:
 * merge buildFields() into the context it writes on every line.
 *
 * This library does not redact. Don't log secrets.
 */

import { instrumentationVersion, version } from './buildinfo.ts';
import { BuildVersion, InstrumentationVersion } from './labels.generated.ts';

/** The build fields to add to every log line. */
export function buildFields(): Readonly<Record<typeof BuildVersion | typeof InstrumentationVersion, string>> {
  return Object.freeze({
    [BuildVersion]: version(),
    [InstrumentationVersion]: instrumentationVersion(),
  });
}
