# instrumentation

Uniform logging and metrics for Milo services, in every language they're
written in.

| Language | Directory | Install |
|---|---|---|
| Go | [go](go) | `go get go.miloapis.com/instrumentation` |

## Shared labels

[labels.yaml](labels.yaml) is the one list of label names every language
generates from. A label is set by your code, set by this library, or reserved
for the collection pipeline. CI fails if any language's generated code differs
from it.

## Releases

Each language is versioned on its own and tagged with its directory as a
prefix: `go/v0.4.0` releases the Go module. A change to `labels.yaml` reaches
each language in its next release.

Go releases before v0.4.0 were tagged at the repository root, from before the
module moved into `go`.

## License

[Apache 2.0](LICENSE)
