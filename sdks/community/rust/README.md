# AG-UI Rust SDK

AG-UI 1.0 types, a framework-independent SSE producer, and an HTTP/SSE client.
The supported compiler floor is Rust **1.88.0** (edition 2024). The checked-in
locks are tested on that released compiler and current stable; the Rust library
CI matrix retains Linux, macOS, and Windows.

## Public boundaries

- `ag-ui-core::processing::normalize_run_input(raw_json)` is the producer's
  pre-admission boundary. It returns canonical `input`, `effective_state`, and
  structured diagnostics. Call it before sending successful HTTP/SSE headers.
- `ag-ui-encoder::Encoder::encode_live(&event)` is the live producer path. It
  validates canonical core events and declares this producer's own `1.0` on
  `RUN_STARTED`. `encode` is stateless and preserves an omitted declaration.
- `ag-ui-client::Agent::run_input(&input, subscribers)` runs the complete
  transport, chunk, lifecycle, reducer, and finalization pipeline with explicit
  identities. Use its complete `messages` and `new_state` for the next turn.
- `Agent::run` exposes a low-level event stream. It does not certify lifecycle
  completion. `run_agent` is a convenience constructor; it creates a fresh thread
  unless its parameters explicitly supply one.

Serde parsing alone is representation parsing, not protocol validation. Tolerant
input/event processing and strict producer-output checking are different APIs.
Unknown closed protocol material may be stripped with a diagnostic; malformed
known values fail. Open application JSON, metadata, opaque values, nested null,
and legal RFC 6902 extension members are preserved.

## Producer integration

1. Bound and decode the incoming JSON request.
2. Call `normalize_run_input`; reject an unsupported major or malformed known
   field before streaming. Apply the host's own authentication/admission policy.
3. Open a response with `ag_ui_encoder::CONTENT_TYPE` and stream
   `Encoder::encode_live` output. Supply the real thread/run identities.
4. Send safe keepalives with `Encoder::keepalive`; close the stream only after
   the appropriate run terminal. The encoder does not enforce event sequencing.

The default encoder limits are 16 MiB per encoded event and 64 KiB per comment,
including framing. `Limits` permits deliberate overrides. Output is bounded
UTF-8, LF-only SSE, without a BOM; oversized values are rejected, never truncated.
No web framework, scheduler, database, authentication system, or server authority
is supplied by these crates.

## Stateful turns, interrupts, and history

Keep one thread ID and choose a new run ID for every run. Carry the previous
public result's `messages`, not `new_messages` or `Replay::message_texts`, so
snapshot replacements and deletions survive. The explicit input also preserves
`parent_run_id`, `context`, `resume`, `tools`, and `forwarded_props`.

Activity is application/UI history and remains in the public result. The HTTP
boundary filters it from conversation history without mutating application
state. Tool results retain their tool-call correlation, typed content, error
content, and metadata. Interrupt responses use one `ResumeEntry` for each open
interrupt. The SDK does not decide whether a tool or approval should execute.

An absent or whole-null optional input state is omitted on the wire and starts
from `{}`. Required `STATE_SNAPSHOT.snapshot: null` remains null, including in
results. On the next run, carry that value normally; wire normalization omits it.
A producer needing canonical null again can send a null snapshot before deltas.
The default JSON state supports scalars and arrays. A custom application state
type can return an explicit conversion failure for values it cannot represent.

`RunAgentResult.terminal_status` distinguishes a finished segment from a
producer `RUN_ERROR`; transport/protocol/subscriber failures return `Err`.
`outcome` further distinguishes success, cancellation, and interruption.
`output: Option<Value>` is separate from terminal status; `result` is its
null-defaulted convenience projection. A producer error is never inferred to be
success merely because the HTTP request returned successfully.

## HTTP, diagnostics, and scope

Both HTTP entry points validate/normalize input just before POST and declare the
consumer's own 1.0 version. JSON `Content-Type` and SSE `Accept` override conflicting
custom values. Responses must be successful HTTP with a real `text/event-stream`
media type; valid MIME parameters/case are accepted.

The incremental decoder tolerates LF, CRLF, CR, an initial BOM, arbitrary UTF-8
chunk splits, multiple frames, and comments. It never dispatches an incomplete
terminal frame or ignores malformed trailing bytes. Default unfinished-line and
frame limits are 16 MiB and 16 MiB + 1024 bytes; `with_sse_limits` overrides them.
Non-success bodies are read for at most 64 KiB or two seconds by default
(`with_error_body_limits`). Their contents are omitted from errors to avoid
reflecting request secrets. Dropping a stream releases local resources; it does
not assert that the remote agent was cancelled.

`HttpAgentBuilder::with_diagnostic_handler` reports processing diagnostics.
Subscribers also receive reducer diagnostics. Categories and RFC 6901 paths are
structured; details are bounded and exclude opaque payloads. Processing is
current AG-UI 1.0, with forward-compatible stripping/warnings. Legacy 0.x era
translation, reconnect/replay execution, protobuf, and WebSockets are excluded.

Core `Replay` is deliberately a limited reducer, not canonical server history.
It supports text, explicit-ID chunks, tool results, full message replacement,
state snapshots, and atomic state patches. It rejects unsupported activity,
reasoning, tool-call assembly, subagent, and custom/raw families. Its
`MESSAGES_SNAPSHOT` behavior is full replacement; use the full client for
role-aware reconciliation.

## Reproducible qualification

From this directory:

```sh
cargo +1.88.0 fmt --all -- --check
cargo +1.88.0 clippy --workspace --all-targets --locked -- -D warnings
cargo +1.88.0 test --workspace --all-targets --locked
cargo +1.88.0 test --workspace --doc --locked
RUSTDOCFLAGS='-D warnings' cargo +1.88.0 doc --workspace --no-deps --locked
```

Repeat the build/test/doc gates on stable. Rust has no Nx project target; the
existing Cargo lane remains direct. The official TypeScript build uses its
existing Nx target. See [real HTTP interoperability](tests/interop/README.md)
and [independent Git consumption](tests/external-consumer/README.md).

The conformance runner reports every manifest case, its classification, and
assertion coverage. Normative Rust overrides are explicit, and legacy exclusions
are distinguished from passing current-1.0 cases. Test counts alone are not a
protocol-conformance claim.

## Git delivery versus registry publication

Use a reachable **immutable 40-hex Git revision** for all three SDK crates and
patch their transitive registry SDK dependencies to that same revision. The
external-consumer recipe resolves an independent lock, then compiles and executes
real host/client probes from a fresh workspace with `--locked`, checks metadata
source identities, and records `cargo tree -d`.

A workspace path patch or `cargo publish --dry-run --no-verify` does not establish
external consumability. The workflow labels unverified client/encoder packaging
as diagnostic. Registry readiness additionally requires ordinary verified
packaging against available registry dependencies, in dependency order. No
crate publication or downstream server integration is implied by Git readiness.
