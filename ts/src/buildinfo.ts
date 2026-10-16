// SPDX-License-Identifier: Apache-2.0

/**
 * The version of the running service and of this library, as reported on
 * instrumentation_build_info and in log fields.
 */

import { readFileSync } from 'node:fs';

const unknown = 'unknown';

let configured: string | undefined;
let frozen: { value: string; stack: string | undefined } | undefined;

/**
 * Sets the service's version. Call it once, at startup, before anything reads
 * the version; afterwards a different value throws, because metrics and logs
 * already carry the first one.
 */
export function setVersion(value: string): void {
  if (frozen && frozen.value !== (value || unknown)) {
    throw new Error(
      `buildinfo: version already read as ${JSON.stringify(frozen.value)}; call setVersion before anything reads it. First read:\n${frozen.stack ?? ''}`,
    );
  }
  configured = value;
}

/**
 * Returns the service's version: the value passed to setVersion, or
 * "unknown". The first call fixes it for the life of the process.
 */
export function version(): string {
  if (!frozen) {
    frozen = { value: configured || unknown, stack: new Error().stack };
  }
  return frozen.value;
}

let instrumentation: string | undefined;

/**
 * Returns this library's version, read from its package.json and prefixed
 * like its release tag, such as ts/v0.1.0, so dashboards tell it apart from
 * the Go module's v0.1.0.
 */
export function instrumentationVersion(): string {
  if (instrumentation === undefined) {
    try {
      const pkg = JSON.parse(readFileSync(new URL('../package.json', import.meta.url), 'utf8')) as { version?: unknown };
      instrumentation = typeof pkg.version === 'string' && pkg.version !== '' ? `ts/v${pkg.version}` : unknown;
    } catch {
      instrumentation = unknown;
    }
  }
  return instrumentation;
}
