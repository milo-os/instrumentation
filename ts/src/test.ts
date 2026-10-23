// SPDX-License-Identifier: Apache-2.0

/**
 * A test assertion that proves a service's metrics go through this library
 * and are served. It reads source rather than inspecting registries at
 * runtime, so it finds every bypass wherever the test lives.
 *
 * Requires the typescript package, which parses the source.
 */

import { readdirSync, readFileSync, statSync } from 'node:fs';
import { extname, join, relative, sep } from 'node:path';

import ts from 'typescript';

const guide = 'https://github.com/milo-os/instrumentation/blob/main/ADOPTING.md';
const seeGuide = `; see ${guide}`;

const client = '@prometheus-io/client';
const metricsModules = new Set(['@miloapis/instrumentation', '@miloapis/instrumentation/metrics']);

const replaced: Readonly<Record<string, string>> = {
  'prom-client':
    "imports prom-client, which was renamed @prometheus-io/client and keeps a registry this library doesn't serve; create metrics with @miloapis/instrumentation/metrics",
  '@hono/prometheus':
    "imports @hono/prometheus, which serves prom-client's registry; serve scrape() and record requests with observeRequest() from @miloapis/instrumentation/metrics",
};

const direct = `imports ${client} directly, so metrics built with it skip the reserved-label check; create them with @miloapis/instrumentation/metrics, which serves them with scrape()`;

const scriptKinds: Readonly<Record<string, ts.ScriptKind>> = {
  '.ts': ts.ScriptKind.TS,
  '.mts': ts.ScriptKind.TS,
  '.cts': ts.ScriptKind.TS,
  '.tsx': ts.ScriptKind.TSX,
  '.js': ts.ScriptKind.JS,
  '.mjs': ts.ScriptKind.JS,
  '.cjs': ts.ScriptKind.JS,
  '.jsx': ts.ScriptKind.JSX,
};

const skippedDirs = new Set(['node_modules', 'dist', 'build', 'coverage', '__tests__']);

/** Options for assertAdopted. */
export interface AdoptedOptions {
  /** The directory to scan. Defaults to the current working directory. */
  root?: string;
  /** Directories under root to leave out, such as a generated client. */
  skip?: readonly string[];
}

/**
 * Throws, listing every finding, unless all of the service's metrics are
 * created and served through @miloapis/instrumentation/metrics. It reports:
 *   - any import of prom-client or @hono/prometheus;
 *   - any value import of @prometheus-io/client, whose metrics skip the
 *     reserved-label check; type-only imports are allowed;
 *   - a service where nothing imports scrape(), so no metric is served.
 *
 * Test files, declaration files, node_modules, dist, build, coverage, and
 * hidden directories are not scanned. Call it from a test under node:test or
 * bun test.
 */
export function assertAdopted(options: AdoptedOptions = {}): void {
  const root = options.root ?? process.cwd();
  const skip = new Set<string>();
  for (const dir of options.skip ?? []) {
    const path = join(root, dir);
    if (!exists(path)) {
      throw new Error(`assertAdopted: skip ${JSON.stringify(dir)} does not exist under ${root}`);
    }
    skip.add(path);
  }

  const findings: string[] = [];
  let scanned = 0;
  let served = false;
  for (const path of sources(root, skip)) {
    scanned++;
    const file = ts.createSourceFile(path, readFileSync(path, 'utf8'), ts.ScriptTarget.Latest, true, scriptKinds[extname(path)]);
    const at = (node: ts.Node, message: string) => {
      const { line } = file.getLineAndCharacterOfPosition(node.getStart(file));
      findings.push(`${relative(root, path).split(sep).join('/')}:${line + 1}: ${message}`);
    };
    const visit = (node: ts.Node): void => {
      const spec = moduleSpecifier(node);
      if (spec !== undefined) {
        const why = replaced[spec.text];
        if (why !== undefined) {
          at(spec, why);
        } else if (spec.text === client && importsValues(node)) {
          at(spec, direct);
        } else if (metricsModules.has(spec.text) && importsScrape(node)) {
          served = true;
        }
      }
      ts.forEachChild(node, visit);
    };
    visit(file);
  }

  if (scanned === 0) {
    throw new Error(`assertAdopted: no source found under ${root}`);
  }
  if (!served) {
    findings.push(`nothing under ${root} imports scrape() from @miloapis/instrumentation/metrics, so no metric created through it is served at /metrics`);
  }
  if (findings.length > 0) {
    throw new Error(findings.map((f) => f + seeGuide).join('\n'));
  }
}

/** The module a node imports, re-exports, or requires, if any. */
function moduleSpecifier(node: ts.Node): ts.StringLiteralLike | undefined {
  if ((ts.isImportDeclaration(node) || ts.isExportDeclaration(node)) && node.moduleSpecifier && ts.isStringLiteralLike(node.moduleSpecifier)) {
    return node.moduleSpecifier;
  }
  if (ts.isImportEqualsDeclaration(node) && ts.isExternalModuleReference(node.moduleReference) && ts.isStringLiteralLike(node.moduleReference.expression)) {
    return node.moduleReference.expression;
  }
  if (ts.isCallExpression(node) && node.arguments.length === 1) {
    const arg = node.arguments[0];
    const isRequire = ts.isIdentifier(node.expression) && node.expression.text === 'require';
    const isDynamicImport = node.expression.kind === ts.SyntaxKind.ImportKeyword;
    if ((isRequire || isDynamicImport) && arg !== undefined && ts.isStringLiteralLike(arg)) {
      return arg;
    }
  }
  return undefined;
}

/** Whether node brings in at least one value, not only types. */
function importsValues(node: ts.Node): boolean {
  if (ts.isImportDeclaration(node)) {
    const clause = node.importClause;
    if (!clause) {
      return true;
    }
    if (clause.isTypeOnly) {
      return false;
    }
    if (clause.name) {
      return true;
    }
    const bindings = clause.namedBindings;
    if (bindings === undefined || ts.isNamespaceImport(bindings)) {
      return true;
    }
    return bindings.elements.some((e) => !e.isTypeOnly);
  }
  if (ts.isExportDeclaration(node)) {
    return !node.isTypeOnly;
  }
  return true;
}

/** Whether node imports scrape, by name or through a namespace import. */
function importsScrape(node: ts.Node): boolean {
  if (!ts.isImportDeclaration(node)) {
    return false;
  }
  const bindings = node.importClause?.namedBindings;
  if (bindings === undefined) {
    return false;
  }
  if (ts.isNamespaceImport(bindings)) {
    return true;
  }
  return bindings.elements.some((e) => (e.propertyName ?? e.name).text === 'scrape');
}

function* sources(dir: string, skip: ReadonlySet<string>): Generator<string> {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) {
      if (!entry.name.startsWith('.') && !skippedDirs.has(entry.name) && !skip.has(path)) {
        yield* sources(path, skip);
      }
    } else if (entry.isFile() && extname(entry.name) in scriptKinds && !isExcludedFile(entry.name)) {
      yield path;
    }
  }
}

/** Whether name is a test or declaration file, such as a.test.ts or a.d.ts. */
function isExcludedFile(name: string): boolean {
  const parts = name.split('.');
  const marker = parts.length >= 3 ? parts[parts.length - 2] : undefined;
  return marker === 'test' || marker === 'spec' || marker === 'd';
}

function exists(path: string): boolean {
  try {
    return statSync(path).isDirectory();
  } catch {
    return false;
  }
}
