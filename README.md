# instrumentation

Uniform logging and metrics for Milo services, in every language they're
written in.

| Language | Directory | Install |
|---|---|---|
| Go | [go](go) | `go get go.miloapis.com/instrumentation` |
| TypeScript | [ts](ts) | `npm install @miloapis/instrumentation` |

To add either one to a service, follow [ADOPTING.md](ADOPTING.md).

## Shared labels

[labels.yaml](labels.yaml) is the one list of label names every language
generates from. A label is set by your code, set by this library, or reserved
for the collection pipeline. CI fails if any language's generated code differs
from it.

## Releases

Each language is versioned on its own and tagged with its directory as a
prefix: `go/vX.Y.Z` for Go and `ts/vX.Y.Z` for TypeScript. A change to
`labels.yaml` reaches each language in its next release. To release, follow
[RELEASING.md](RELEASING.md).

## License

[Apache 2.0](LICENSE)
