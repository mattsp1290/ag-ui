# AG-UI Go Example Server

This example is a Fiber v3 AG-UI server for local development and Dojo testing.
It replaces the older minimal `/agentic` example with a broader route surface
based on the Go Dojo server example. It pairs with the eleven-page [Flutter dojo](../../../dart/example).

## Run

Requires Go 1.25 or newer. Fiber v3.3.0 requires Go 1.25, so `go mod tidy`
updates this example module to `go 1.25.0`.

```bash
cd sdks/community/go/example/server
# OPENAI_API_KEY must already be set in this terminal.
go run ./cmd
```

The default server address is `http://127.0.0.1:8080`.

The server uses the reproducible `MODEL_PROVIDER=openai` path from
`github.com/cloudwego/eino-ext/components/model/openai`. Prototype-only provider
and tool dependencies from the standalone source were removed for this monorepo
example. The read-only `file_read` tool is implemented locally in
`internal/agent/tools.go`.

## Environment

| Variable | Default | Description |
| --- | --- | --- |
| `HOST` | `127.0.0.1` | Server bind host. |
| `PORT` | `8080` | Server port. |
| `MODEL_PROVIDER` | `openai` | Only `openai` is supported in this monorepo example. |
| `MODEL` | `gpt-4o` | Model passed to the OpenAI Eino provider. |
| `OPENAI_API_KEY` | unset | Required at normal executable startup; only the test fixture starts without it. |
| `AGENT_WORKSPACE` | process working directory | Read-only root for the `file_read` tool. Set it deliberately. |
| `AGENT_AUTO_APPROVE` | `false` | Bypass the approval interrupt path for `/agentic`. |
| `AGENT_MAX_ITERATIONS` | `8` | Model/tool loop iteration budget. |
| `CORS_ENABLED` | `true` | Permissive CORS for local UI development. |
| `AGENTIC_UI_PACE_MS` | `600` | Chunk/step delay for scripted checklist and reasoning routes. |

## Routes

| Route | Notes |
| --- | --- |
| `GET /` | Health/config metadata and route list. |
| `POST /agentic` | Primary read-only agent loop. |
| `POST /agentic_chat` | Dojo frontend-tools chat route. |
| `POST /backend_tool_rendering` | Dojo route alias using the frontend-tools chat posture. |
| `POST /human_in_the_loop` | Approval route. `X-AG-Approval: off` or `?approval=off` disables the approval gate. |
| `POST /agentic_generative_ui` | Deterministic multi-step generative UI route. |
| `POST /tool_based_generative_ui` | Client-tool generative UI route. |
| `POST /shared_state` | Recipe shared-state route. |
| `POST /predictive_state_updates` | Document predictive state updates route. |
| `POST /agentic_chat_multimodal` | Multimodal-capable chat route using the OpenAI provider path. |
| `POST /image-gen` | OpenAI image generation helper. |
| `POST /vision` | OpenAI vision helper. |
| `POST /audio` | OpenAI audio transcription helper. |
| `POST /document` | OpenAI document analysis helper. |
| `POST /reasoning` | Scripted balanced reasoning and answer events with stable snapshot IDs. |

The checklist and reasoning routes are scripted. Other feature routes use the configured provider; frontend approval is a local demonstration, not execution of the described action. Navigation in Flutter starts a fresh page/thread and state is ephemeral.

All `POST` routes accept AG-UI `RunAgentInput` JSON unless noted by their
specific handler. SSE responses use the AG-UI event stream format.

## Dojo

Run this server separately and configure Dojo to point the relevant integration
runtime URL at `http://127.0.0.1:8080`. This server lives under the Go SDK
examples rather than an `integrations/.../examples` package, so it is not
automatically registered as a first-class Dojo integration by this change.

## Go Client

The Go example client default endpoint is aligned with this server:

```text
http://localhost:8080/agentic
```

Pass an explicit endpoint to the client if you run the server on a different
host or port.

## Container

Build from the **Go SDK root**, so the nested module's `replace ../../` includes the current parent SDK:

```bash
# Repository root:
cd sdks/community/go
docker build -f example/server/Dockerfile -t ag-ui-go-example:local .
# OPENAI_API_KEY is already set; the value is passed only at runtime.
docker run --rm -p 127.0.0.1:8080:8080 -e OPENAI_API_KEY \
  -e AGENT_WORKSPACE=/tmp/empty --tmpfs /tmp/empty ag-ui-go-example:local
```

The image runs `/app/server` as a non-root user, binds `0.0.0.0:8080` inside the container, and accepts `HOST`, `PORT`, and the environment settings above. It retains CA certificates for provider HTTPS. The explicit empty workspace bounds the read-only tool. No credentials are baked into the image. SIGTERM cancels request contexts and lets the server shut down gracefully.

The separate `Dockerfile.contract` compiles a build-tagged Go test entrypoint from the same `cmd` package. It constructs production `newApp`, uses a fresh store/empty workspace and scripted Eino/media callbacks, and never contacts a provider. It runs only `TestServeFlutterContract`, with an 18-minute graceful lifetime under the test binary's 20-minute ceiling. This test-only entrypoint is absent from normal builds; there is no production fake-provider mode.

When refreshing the pinned builder or runtime digests, rebuild both Dockerfiles and rerun the production startup/non-root checks and the contract suite.

## Credential-free pairing and verification

The [Flutter README](../../../dart/example) documents `dart run test/helpers/go_server_container.dart`, which builds and owns the deterministic Docker fixture and prints its dynamically mapped URL. Pass that URL in Flutter's `--dart-define=AG_UI_BASE_URL=...`. The ordinary default pair uses `http://127.0.0.1:8080`; changing Go `PORT` requires a matching Flutter compile-time define. Provider keys stay exclusively in the Go environment.

```bash
# sdks/community/go/example/server:
GOWORK=off go mod download
GOWORK=off go build ./...
GOWORK=off go test ./...
GOWORK=off go test -race ./...
GOWORK=off go mod tidy -diff
```

Run parent SDK regressions separately with `go test ./...` from `sdks/community/go`. The Flutter pairing gate is owned by Dart:

```bash
# sdks/community/dart/example, with Docker Engine/CLI available:
flutter pub get --enforce-lockfile
flutter test test/go_server_contract_test.dart --tags requires-go-server --timeout 15m
```

It tests the contract image and the normal image's missing-key error, sentinel-key health/reasoning, non-root entrypoint, bind address, and graceful shutdown. The sentinel is not a usable credential and is never sent to a provider. Clean-checkout Flutter web/macOS builds and parent SDK regression tests passed. Required interactive Chrome/macOS observations remain pending: Chrome is not connected, and the native launch exposed no window to accessibility automation. Deterministic tests do not establish interactive platform or live-provider success.

Live-provider follow-up is unperformed, owner Matt. From the repository root, run `cd sdks/community/go/example/server` then `GOWORK=off go test -tags integration ./...` with valid server-side credentials for existing opt-in provider tests; a default `go test ./...` does not execute them. Record actual results without credential values.

## Source reconciliation

Compared with `mattsp1290/ag-ui-go-server-example` at `ad756b7afd1c9abc4ebf422aaa28d338d154d2c3`. The standalone checkout is provenance only and remains intact.

| Source area | Disposition |
| --- | --- |
| `cmd/server/main.go` | Existing import retained at `cmd/main.go`; preserved destination `newApp`, CORS/health metadata, `/backend_tool_rendering`, `/agentic_chat_multimodal`, graceful shutdown and shared error handling. Added `/reasoning`. |
| Agent checklist/state/document helpers | Existing source behavior retained. |
| Agent loop and shared state | Retained bounded iterations, frontend tools and state merging; added application-owned wire history so incoming, streamed, tool-owner and result IDs survive snapshots and resume. |
| Conversion/emitting/run configuration | Destination-owned `convert.go`, `emitter.go`, `loop.go`, and `runconfig.go` replace the source-only `agui.go` alias. Provider-only encrypted data remains scrubbed from wire snapshots. |
| Predictive updates | Retained destination step/error ordering and cancellation fixes. |
| Model/config | Retained reproducible OpenAI provider; excluded standalone subscription credentials, `codex-auth-go`, and placeholder/local provider dependencies. |
| Read-only tools | Preserved local workspace-bound `file_read`; excluded the source-only `eino-tools` adoption. |
| Run store | Retained capacity/TTL/ownership behavior; cloned wire history added for paused/resumed runs. `internal/wireclone` is a shared destination helper. |
| Media provider clients and fixtures | Existing clients and intentional audio/PDF fixtures retained; no provider-client redesign. |
| Media handlers | Preserved production extraction/SSE/result handling; added per-handler provider callbacks and restricted input selection to the newest user turn. |
| Source tests | Existing tests retained with destination imports; expanded route, malformed-input, CORS, identity, ownership, iteration, predictive-error, reasoning and media-boundary regressions. |
| Standalone module files and launcher | Excluded absolute/sibling replacements and source-only `eino-agui`, `eino-providers`, and `eino-tools` dependencies. The only local module replacement remains the parent SDK at `../../`. |

Public SDK APIs and implementations are unchanged by the example migration. Revert related example/CI/documentation commits together if the pair must be rolled back; there is no durable example data migration.


## Go/Python/TypeScript parity

The SDK [parity gate](../../README.md#verify-a-checkout) checks real codecs and
SSE producers from all three language checkouts, independently of the Flutter
pairing suite above. Run `GOWORK=off bash scripts/go-sdk-parity.sh` at the
repository root after installing its pinned peer dependencies.

The example client uses the canonical event decoder for reasoning, subagent,
activity and existing event families. The scripted reasoning route emits role
`reasoning`, as required by the current wire schema. Existing deterministic
server scenarios remain part of the nested-module regression suite.

Both Dockerfiles and Flutter's temporary build-context copier include the
SDK's runtime `internal/jsonnumber` package. Recheck both direct image builds
and the Flutter contract suite when changing SDK runtime dependencies.
