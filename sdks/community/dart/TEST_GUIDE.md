# Testing Guide for AG-UI Dart SDK

The SDK package is version 0.3.0 and supports Dart `>=3.3.0 <4.0.0`. The
Flutter example is a separate package with its own lockfile and currently uses
Flutter 3.47.1 (Dart 3.13.1), as pinned by `.github/workflows/unit-dart-sdk.yml`.

There is no Dart project or target in the repository's Nx graph. Repository
automation therefore uses the direct Dart and Flutter commands below; Nx is
used for the JavaScript/TypeScript projects only.

## Running Tests

### Default suite (Recommended)

Run the full SDK suite. Set `AGUI_SKIP_DOJO=1` to skip the live Dojo cases
explicitly; the current SDK tests do not use a `requires-server` tag.

```bash
AGUI_SKIP_DOJO=1 dart test
```

The public documentation examples have an executable API and exhaustiveness
probe:

```bash
dart test test/parity/documentation_test.dart
```

### All Tests

To run the full Dart test tree:

```bash
dart test
```

### Browser model tests
The CI browser lane runs the pure model tests explicitly so it never compiles the
`dart:io` integration helpers:

```bash
dart test --platform chrome \
  test/types/metadata_test.dart \
  test/types/run_outcome_test.dart \
  test/types/subagent_outcome_test.dart \
  test/types/token_usage_test.dart \
  test/types/capabilities_test.dart
```

### Cross-language parity
From the repository root, run the Dart producer/consumer parity gate with:

```bash
bash scripts/dart-sdk-parity.sh
```

The gate accepts these optional controls. They are caller-owned and are never
silently mapped onto or used to override the existing `AG_UI_PARITY_*` variables
used by the Go parity harness:

| Variable | Meaning |
| --- | --- |
| `AG_UI_DART_PARITY_OUTPUT_DIR` | Temporary directory for generated artifacts. The caller owns its lifecycle. |
| `AG_UI_DART_PARITY_PHASE` | Explicit `produce`, `consume`, or `verify` phase for targeted runs. |
| `AG_UI_DART_PARITY_CORPUS` | Explicit path to the resolved shared parity corpus. |

For example, to retain artifacts for inspection:

```bash
AG_UI_DART_PARITY_OUTPUT_DIR="$PWD/.dart-parity-output" \
  bash scripts/dart-sdk-parity.sh
```

## Test Categories

### Unit Tests
- **SSE Components**: Parser, client, messages, backoff strategies
- **Types**: Base types, messages, tools, context
- **Encoder/Decoder**: Client codec, error handling
- **Events**: Event types, event handling
- **Client**: Configuration, error handling

### Integration Tests

`event_decoding_integration_test.dart` and `fixtures_integration_test.dart`
exercise local fixtures. The `dojo_*` tests exercise decoding, smoke, and
resilience behavior against `AGUI_DOJO_BASE_URL` or `AGUI_BASE_URL`; live cases
skip when the Dojo is unavailable or when `AGUI_SKIP_DOJO=1`.

## Parity baseline

`test/fixtures/parity_manifest.json` is the resolved inventory and provenance
record. Its work packages, test catalog, and verification surfaces identify
the executable evidence for each slice. The 95-case shared corpus is exercised
over 16 total cross-language artifact routes by
`test/parity/codec_routes_test.dart` and `scripts/dart-sdk-parity.sh`. The
manifest pins implementation base
`aaa75b54d572be8cd1d51c72e951273c5b893ed0` and upstream inspection revision
`0fa1bebd9772de79347f0caf79744535e94ec37c`.

`test/fixtures/compatibility.json` and
`test/parity/compatibility_test.dart` retain the original public call shapes,
JSON aliases and defaults, explicit null behavior, cipher-safe `rawEvent`
handling, and no-default exhaustive-switch probes. The current protocol model
has 36 canonical events plus the deprecated Dart-only `THINKING_CONTENT`
compatibility case.

Resolve dependencies with `dart pub get --no-example`, then run the suite,
documentation, and parity commands above. For local static checks, use:

```bash
dart analyze lib
dart analyze test
dart format --output=none --set-exit-if-changed lib test
```

The analyzer and whole-tree format commands may continue to report drift
pinned by the original baseline;
changed Dart files must still be formatted. The Flutter package has its own
analysis, test, Docker contract, and build gates in `example/README.md`.

## Known Limitations

1. **SSE Retry Tests**: Two tests are skipped because SSE protocol doesn't support automatic retry on HTTP errors - this is a protocol limitation, not a bug.

2. **Live Dojo Tests**: Need a reachable compatible Dojo server; otherwise the
live cases report skips.

3. **Transport and runtime scope**: Protobuf event encoding, WebSockets,
capability discovery, and the TypeScript-specific agent/middleware/reactive
runtimes are not part of the Dart parity gate.

## Bounded-byte-parser conformance

```bash
dart test test/sse/
dart test test/client/http_endpoints_test.dart
dart test --platform chrome test/sse/sse_client_stream_test.dart test/sse/sse_client_basic_test.dart
dart analyze tool/verify_sse_byte_bounds.dart tool/verify_sse_logging.dart
dart run tool/verify_sse_byte_bounds.dart
dart --enable-vm-service=0 run tool/verify_sse_logging.dart
```

The stream suite imports the public barrel. It keeps controllers open through
L+1 failure, checks all field classes in oversized and fragmented chunks,
compares explicit fixtures and the old decoding-chain oracle for bounded valid
input, and observes pause/resume/cancel before any message, during UTF-8 and
CRLF, and across independent calls. The separate VM HTTP suite disables server
response buffering to prove failure before the response is released, then
reuses the same borrowed HTTP client. Controller tests deterministically assert
one upstream cancellation; an HTTP socket disconnect is not required for client
reuse. Chrome runs exclude the `dart:io` HTTP harness.

The logging probe subscribes to actual VM-service `Logging` events, exercises
all overflow field classes and the bounded-but-dropped ID log, and waits for a
sentinel log before checking synthetic canaries. It also captures Zone prints;
Zone interception alone does not observe `developer.log`. Do not run probes
with real prompts or credentials. The byte probe uses runtime checks (not Dart
assertions), the public barrel and SDK libraries only.

### Fresh public immutable dependency

After pushing the corrected commit, run this from the package directory. Set
`SSE_PIN` to the **full actual corrected Git SHA**. Do not use the original
`cce5da216ed936902e703ba4317d206832ff8eee` pin. The directory and cache must be
new and outside the checkout. The recipe disables Git credential/config
sources and interactive prompts; it uses neither local paths nor overrides.

```bash
SSE_CONSUMER=$(mktemp -d)
SSE_CACHE=$(mktemp -d)
mkdir -p "$SSE_CONSUMER/bin"
cp tool/verify_sse_byte_bounds.dart "$SSE_CONSUMER/bin/"
cat > "$SSE_CONSUMER/pubspec.yaml" <<EOF_PUB
name: bounded_sse_consumer
environment:
  sdk: '>=3.3.0 <4.0.0'
dependencies:
  ag_ui:
    git:
      url: https://github.com/mattsp1290/ag-ui.git
      ref: $SSE_PIN
      path: sdks/community/dart
EOF_PUB
cd "$SSE_CONSUMER"
env -u GITHUB_TOKEN -u GH_TOKEN -u GIT_CONFIG_PARAMETERS \
  PUB_CACHE="$SSE_CACHE" GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null \
  GIT_CONFIG_COUNT=0 GIT_TERMINAL_PROMPT=0 GIT_ASKPASS=/usr/bin/false \
  SSH_ASKPASS=/usr/bin/false dart pub get
PUB_CACHE="$SSE_CACHE" dart analyze
PUB_CACHE="$SSE_CACHE" dart run bin/verify_sse_byte_bounds.dart
```

Inspect `pubspec.lock`: `ag_ui.description.resolved-ref` must equal `SSE_PIN`.
Inspect `.dart_tool/package_config.json`: `ag_ui.rootUri` must point inside
`SSE_CACHE`, not a workspace. Retain the exact SHA, probe Git blob hash, tool
versions, command exit statuses and redacted logs with the owner response.
The public pin proves availability and conformance; consumer adoption and its
plan's dependency/readiness updates remain the consumer owner's separate step.
