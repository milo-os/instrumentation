// SPDX-License-Identifier: Apache-2.0

/**
 * Label names shared by every service, for metric labels and log keys alike.
 *
 * Subject labels describe what a signal is about, and code sets them. Build
 * labels describe the binary, and this library sets them. Reserved labels are
 * set by the collection pipeline: a metric or log line that sets one collides
 * with the pipeline's value, and Prometheus renames it to exported_<name>.
 *
 * The names come from labels.yaml at the repository root, which every
 * language in the repository generates from.
 */

import { reserved } from './labels.generated.ts';

export * from './labels.generated.ts';

/** Returns why name is reserved for the collection pipeline, or undefined. */
export function isReserved(name: string): string | undefined {
  return Object.hasOwn(reserved, name) ? reserved[name] : undefined;
}

/** Returns every reserved label name, sorted. */
export function reservedLabels(): string[] {
  return Object.keys(reserved).sort();
}
