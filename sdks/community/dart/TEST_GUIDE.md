# Testing Guide for AG-UI Dart SDK

The SDK package is version 0.3.0 and supports Dart `>=3.3.0 <4.0.0`. The
Flutter example is a separate package with its own lockfile and currently uses
Flutter 3.47.1 (Dart 3.13.1), as pinned by `.github/workflows/unit-dart-sdk.yml`.

There is no Dart project or target in the repository's Nx graph. Repository
automation therefore uses the direct Dart and Flutter commands below; Nx is
used for the JavaScript/TypeScript projects only.

## Running Tests

### Unit Tests Only (Recommended)
Run unit tests excluding integration tests that require external services:

```bash
dart test --exclude-tags requires-server
```

### All Tests
To run all tests including integration tests (requires TypeScript SDK server setup):

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

### Unit Tests (381+ tests) ✅
- **SSE Components**: Parser, client, messages, backoff strategies
- **Types**: Base types, messages, tools, context
- **Encoder/Decoder**: Client codec, error handling
- **Events**: Event types, event handling
- **Client**: Configuration, error handling

### Integration Tests
These tests require the TypeScript SDK's Python server to be running:
- `simple_qa_test.dart` - Tests Q&A functionality
- `tool_generative_ui_test.dart` - Tests tool-based UI generation
- `simple_qa_docker_test.dart` - Docker-based integration tests

**Note**: Integration tests are tagged with `@Tags(['integration', 'requires-server'])` and will be skipped by default when using `--exclude-tags requires-server`.

## Test Coverage

The SDK has comprehensive unit test coverage including:
- 6 SSE client basic tests
- 8 SSE stream parsing tests
- 13 SSE message tests
- 67 base types and JSON decoder tests
- 39 error handling tests
- 59 event type tests
- 23 client configuration tests
- And many more...

## Parity baseline

`test/fixtures/compatibility.json` records the exact base commit, current
public event and message-role surface, and the compatibility cases that must
remain green while Dart parity is implemented. `test/parity/compatibility_test.dart`
exercises those cases, including the 34 current event variants (the
Dart-only `THINKING_CONTENT` is recorded separately), constructor and
`copyWith` call shapes, JSON aliases/defaults, explicit nulls, cipher-safe
`rawEvent` handling, run inputs, and the no-default exhaustive-switch compile
probes. Rows for features added by later parity slices are deliberately not
present as failing future tests.

The baseline was run on the pinned base `aaa75b54d572be8cd1d51c72e951273c5b893ed0`.
`dart pub get --no-example` and `dart test --exclude-tags requires-server` pass
on Dart 3.13.1. The SDK analyzer commands currently report pre-existing
warnings/TODOs (`dart analyze lib`: 11 warnings; `dart analyze test`: 42
warnings), and `dart format --output=none --set-exit-if-changed lib test`
reports 13 existing files requiring formatting. These are recorded baseline
drift and are not part of the compatibility PR. The Flutter example currently
passes `flutter analyze`, ordinary `flutter test --exclude-tags
requires-go-server` (109 tests), and `flutter build web`; its format check
reports the existing `test/go_server_contract_test.dart` drift.

## Known Limitations

1. **SSE Retry Tests**: Two tests are skipped because SSE protocol doesn't support automatic retry on HTTP errors - this is a protocol limitation, not a bug.

2. **Integration Tests**: Require TypeScript SDK infrastructure that may not be available in the Dart SDK directory structure.
