# AGENTS.md

Guidance for coding agents working in this repository.

## What this is

Uniform logging and metrics for Milo services, in two languages that share one
label list:

| Path | What |
|---|---|
| `labels.yaml` | The shared label names. The only source of truth. |
| `go/` | Go module `go.miloapis.com/instrumentation` |
| `ts/` | npm package `@miloapis/instrumentation` |
| `go/internal/labelgen` | Generates both languages' label code |

## Commands

Run from the repository root.

| Command | Does |
|---|---|
| `task ci` | Everything GitHub CI runs. Run it before every push. |
| `task generate` | Regenerate both languages' label code from `labels.yaml` |
| `task go:ci`, `task ts:ci` | One language's checks |

`task ci` and GitHub CI run the same tasks, so a change that passes locally
passes in CI.

## Rules

- Never hand-edit `go/labels/zz_generated.go` or
  `ts/src/labels.generated.ts`. Edit `labels.yaml`, then run `task generate`.
- A change to `labels.yaml` reaches each language only in that language's next
  release.
- Go commands in Taskfiles set `GOTOOLCHAIN` inline. Task's `env:` doesn't
  override a variable already set, and CI sets `GOTOOLCHAIN=local`.
- Keep the Go floor at Go 1.25.0 and controller-runtime v0.20.4, so services
  pinned to older Kubernetes libraries can still adopt the module.
- The TypeScript package builds on `@prometheus-io/client`, never
  `prom-client`, and runs on Node.js 24 and Bun. Its tests run under both.
- Every pull request needs an issue in this repository that it closes.

## Releases

Each language is versioned and tagged on its own: `go/vX.Y.Z` and `ts/vX.Y.Z`.
Before releasing, read [RELEASING.md](RELEASING.md) and follow it exactly. A
TypeScript release ends with an approval on npm that only a person can give.
