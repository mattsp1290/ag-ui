# AG-UI Go SDK

Go models, event codecs, an SSE client and an SSE server writer for AG-UI.
See the [Go overview](../../../docs/sdk/go/overview.mdx),
[events](../../../docs/sdk/go/core/events.mdx),
[types and helpers](../../../docs/sdk/go/core/types.mdx),
[encoding](../../../docs/sdk/go/encoding/overview.mdx) and
[SSE client](../../../docs/sdk/go/client/sse-client.mdx) guides.

The checkout includes parity additions that may not yet be published in a Go
module release. The overview records additions, invalid-wire corrections,
known peer differences and the inherited ToolCall source-compatibility break.
The [manifest](testdata/parity/manifest.json) and
[baseline](testdata/parity/baseline.md) are the executable contract inventory and
historical compatibility evidence.

## Verify a checkout

From the repository root, use its Node/pnpm/Python pins and each module's Go
version. Install the peer SDKs from this checkout, then run the complete gate:

```bash
pnpm install --frozen-lockfile
(cd sdks/python && uv sync --locked)
GOWORK=off bash scripts/go-sdk-parity.sh
```

This checks all directed core routes and actual Go/Python/TypeScript SSE
scenarios using fresh temporary output. Missing runtimes or artifacts fail;
Nx caching is disabled for the oracle calls. For only the SSE scenarios, run
`GOWORK=off bash scripts/go-sdk-sse-parity.sh` after the same dependency setup.

Go's four modules must also be checked separately; SDK-root `./...` does not
include nested modules:

```bash
# Repository root:
for module in sdks/community/go sdks/community/go/example/client \
  sdks/community/go/example/server integrations/community/genkit/go/genkit; do
  (cd "$module" && GOWORK=off go build ./... && \
    GOWORK=off go test -race ./... && GOWORK=off go vet ./...) || exit 1
done
```

Run `GOWORK=off go mod tidy -diff` in each nested consumer module to verify its
module files. Selected external API and unchanged wire fixtures run with
`GOWORK=off go test ./internal/parity` from this SDK directory.

The [example server](example/server/README.md) documents Docker builds and the
credential-free Flutter contract suite. Live-provider runs are optional and
separate from the parity gate.
