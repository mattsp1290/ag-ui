# Bounded SSE byte parser verification

Implementation base: PR #45, `aec2278f64257d6c7141256648eb84cb1c6c3495`,
`plan-pr/fd9437a7322240a8/09-dart-docs`. Checked September 9, 2026 in an isolated
worktree. No protocol models, caller history, dependencies or lockfiles changed.
See README for API, compatibility and the algebraic payload bound, and
TEST_GUIDE for reproducible commands including the anonymous public-pin probe.

## Requested-base baseline

Dart 3.9.3 on macOS arm64, Chrome; `dart pub get --no-example` succeeded.
SSE tests: 58 passed. HTTP endpoint tests: 18 passed. Chrome stream tests:
8 passed. Full suite with `AGUI_SKIP_DOJO=1` and `--exclude-tags requires-server`:
863 passed, 9 skipped. Documentation tests: 5 passed.

Inherited static-check results (not treated as passing):

- `dart analyze lib`: exit 2, 328 diagnostics.
- `dart analyze test`: exit 2, 1048 diagnostics.
- `dart format --output=none --set-exit-if-changed lib test`: exit 1,
  83 files inspected, 9 with inherited formatting differences.

## Implemented-head local evidence

- Focused SSE plus HTTP endpoint suite: 122 passed (exit 0).
- Chrome public byte/controller/constructor and existing five model files:
  109 passed (exit 0).
- Full SDK suite: 909 passed, 9 existing/live-server skips (exit 0).
- Documentation tests: 5 passed (exit 0).
- Public byte probe: PASS for open L+1 bounds, exact limits, aggregation and
  cancellation (exit 0).
- VM-service Logging and Zone print capture: PASS, including the dropped-ID log
  and completion sentinel (exit 0).
- New Dart files and both tools: no diagnostics. Existing lib diagnostics reduce
  to 327; test diagnostics remain the baseline 1048. Comparison by file and
  diagnostic message confirms no additions. Changed-file format: 12 files,
  zero differences (exit 0). Whole-tree inherited format drift remains.
- `bash scripts/dart-sdk-parity.sh`: verified 16 routes × 95 cases (exit 0),
  including actual Go-produced SSE. Used the existing Nx TypeScript targets.
- Flutter 3.47.1 / Dart 3.13.1: enforced lockfile resolution, analysis and
  Flutter-SDK formatter passed; 129 ordinary tests passed; web build passed.
- Docker Engine 28.3.3: Flutter Go contract tests passed (15); Docker failure
  tests passed (2); assertion/readiness failure cleanup verifier passed.

The default `dart` on this host is 3.9.3; the Flutter example's format gate uses
its pinned Dart 3.13.1. Running the older formatter there reports an inherited
one-file difference. No unrelated formatting cleanup is included.

Controller tests prove cancellation exactly once, including before input,
with partial UTF-8/CRLF and while paused. A single huge chunk pauses after its
first frame without processing later frames or an invalid suffix. Loopback
HTTP holds an unterminated response open with server buffering disabled, observes
failure before release, cancels, and performs another request through the same
borrowed client. Socket identity is not part of that contract.

All original Dart workflow jobs are retained. The workflow additionally runs
Chrome framing/lifecycle tests and both public-byte/VM-logging probes. Published
commit resolution and fresh-consumer evidence must be recorded against the real
full SHA after push in the SDK owner's response; this document does not invent
a release or a public pin. Consumer adoption remains a separate owner action.
