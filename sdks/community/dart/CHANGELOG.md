# Changelog

All notable changes to the AG-UI Dart SDK will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- `TimeoutError` renamed to `AGUITimeoutError` to avoid shadowing the
  built-in `dart:async.TimeoutError` (raised by `Future.timeout(...)` /
  `Stream.timeout(...)`). The bare name is preserved as a deprecated
  typedef alias for backward compat and will be removed in 1.0.0.
  Internal call sites in `AgUiClient` throw the new name directly. The
  README "Errors" recipe and "Migrating from 0.1.0" section call out
  the rename so consumers using both `package:ag_ui/ag_ui.dart` and
  `dart:async` can avoid the symbol collision.
- Empty `delta` is now accepted on `TEXT_MESSAGE_CONTENT`,
  `TOOL_CALL_ARGS`, and `REASONING_MESSAGE_CONTENT`, and empty
  `content` is accepted on `TOOL_CALL_RESULT`, to match the canonical
  TS/Python schemas (`z.string()` / `str` with no `min(1)` constraint).
  Previously the Dart SDK rejected empty values at both the `fromJson`
  factory and the `EventDecoder.validate` pipeline; a Python or TS
  server that legitimately emitted a deliberate empty chunk (e.g. a
  noop content refresh) would fail decode in Dart but pass in the
  canonical SDKs. Empty cipher payloads on `REASONING_ENCRYPTED_VALUE`
  (`entityId`, `encryptedValue`) continue to be rejected — the "no
  graceful default for cipher payloads" contract stays.

### Fixed
- `ToolCall` now carries the optional `encryptedValue` field for parity
  with canonical TS (`ToolCallSchema.encryptedValue: z.string().optional()`)
  and Python (`ToolCall.encrypted_value: Optional[str]`). Previously a
  message arriving with `toolCalls: [{..., encryptedValue: "..."}]`
  silently dropped the value at decode and could not re-emit it on a
  proxy hop. Decode accepts both `encryptedValue` and `encrypted_value`;
  `toJson` emits the camelCase key when present; `copyWith` uses the
  sentinel pattern so callers can explicitly clear it via
  `copyWith(encryptedValue: null)`.
- `RunAgentInput` now carries the optional `parentRunId` field for
  parity with canonical TS (`RunAgentInputSchema.parentRunId:
  z.string().optional()`) and Python (`RunAgentInput.parent_run_id`).
  Previously a `RUN_STARTED` payload with `input.parentRunId: '...'`
  decoded with the field silently dropped, even though
  `RunStartedEvent.parentRunId` itself was preserved. Decode accepts
  both `parentRunId` and `parent_run_id`; `toJson` emits camelCase when
  present; `copyWith` uses the sentinel pattern.
- `EventStreamAdapter.fromRawSseStream` now handles CRLF (`\r\n`) line
  terminators, not just LF. Previously a CRLF-emitting SSE server
  produced `"\r"` lines that never matched the empty-line event-boundary
  signal, so events buffered until stream close. The line splitter now
  strips a trailing `\r` after splitting on `\n`. The same fix is
  applied to `EventDecoder.decodeSSE`, which now uses `LineSplitter`
  (handling `\n`, `\r`, and `\r\n` per the WHATWG SSE spec).
- `JsonDecoder.optionalListField` and `requireListField` now eagerly
  type-check elements (raising `AGUIValidationError(field: '$field[$i]')`
  on the first wrong-typed element) instead of returning a lazy
  `cast<T>()` view that surfaced as a raw `TypeError` at access time and
  was flattened to `field: 'json'` by the decoder catch-all.
- `AssistantMessage.fromJson` now uses `JsonDecoder.optionalEitherField`
  on the `toolCalls` / `tool_calls` key itself, instead of a `??` chain
  on the post-`.map(...).toList()` value. The previous chain only fired
  on null, so an empty `toolCalls: []` short-circuited the snake_case
  fallback even when `tool_calls: [...]` was populated.
- `AssistantMessage.toJson` now emits `toolCalls` whenever the in-memory
  field is non-null (including empty lists), so the round-trip
  `fromJson(m.toJson()) == m` is symmetric.
- Decoder pipeline now rethrows `EncoderError` / `DecodeError` /
  `EncodeError` unchanged instead of re-wrapping them as a generic
  "Failed to decode event" via the catch-all.
- `EventEncoder.encodeSSE` no longer strips fields whose value is `null`.
  The blanket `json.removeWhere((k, v) => v == null)` was silently
  dropping fields that intentionally serialize as `null`
  (`ActivitySnapshotEvent.content`, `RawEvent.event`, `CustomEvent.value`,
  `StateSnapshotEvent.snapshot`), breaking the encode→decode round-trip
  because the matching factories require the key to be present and reject
  it with `AGUIValidationError`. Each `toJson()` already uses
  `if (field != null) 'field': field` for fields that opt in to omission,
  so the strip pass was redundant in addition to harmful. Pinned by a
  new round-trip test in `fixtures_integration_test.dart`.
- `EventStreamAdapter.fromRawSseStream` now handles WHATWG-spec lone-`\r`
  line terminators in addition to `\n` and `\r\n`. The previous chunk
  scanner only split on `\n`, so a producer using bare `\r` (rare in
  practice but spec-valid) buffered indefinitely. The new multi-terminator
  scanner defers a trailing `\r` at chunk boundaries to disambiguate from
  a chunk-spanning `\r\n` and consumes it on stream close. Steady-state
  emission for CRLF-encoded streams is unchanged.
- `EventStreamAdapter.fromSseStream` and `fromRawSseStream` now preserve
  any `AGUIError` subtype (`AgUiError`, `AGUIValidationError`,
  `EncoderError`) raised by the decoder instead of re-wrapping the
  encoder-family errors as a generic `DecodingError`. Mirrors the
  unified-error-surface contract that `EventDecoder.decode/decodeJson`
  already honor.
- `TestHelpers.findToolCalls` (test-only helper) now uses the typed
  `AssistantMessage.toolCalls` accessor. Previously it round-tripped
  through `toJson` and read the snake_case key `tool_calls`, but
  `AssistantMessage.toJson` emits camelCase `toolCalls` — the helper
  silently always returned an empty list. Currently unreferenced by the
  test suite, so this is a latent-bug fix.

### Added
- `JsonDecoder.optionalEitherListField<T>` helper combining the dual-key
  resolution rule from `optionalEitherField` with the index-aware
  element-type validation from `requireListField` / `optionalListField`.
  `AssistantMessage.fromJson` now uses it so a malformed nested
  `toolCalls[i]` raises `AGUIValidationError(field: 'toolCalls[$i]')`
  instead of leaking a raw `TypeError` from the per-element cast.

### Changed
- `Message` subclass `copyWith` methods (`DeveloperMessage`,
  `SystemMessage`, `UserMessage`, `AssistantMessage`, `ToolMessage`,
  `ReasoningMessage`) now use the `_unsetMessage` sentinel pattern for
  nullable fields, matching the event-class discipline. Callers can
  explicitly clear a nullable field via `copyWith(field: null)` —
  previously `?? this.field` could not distinguish "argument omitted"
  from "argument explicitly null".
- `JsonDecoder.optionalIntField` (new helper) accepts `int` or `num`
  and coerces via `.toInt()`. Every event factory now reads
  `timestamp` via this helper, so a TS server emitting a fractional
  number (e.g. `Date.now() / 1000`) no longer fails decode with
  `AGUIValidationError(field: 'timestamp')`.
- Error-hierarchy unification: `AgUiError` now extends `AGUIError`,
  and `AGUIValidationError` now extends `AGUIError` instead of bare
  `implements Exception`. Callers can `on AGUIError catch (e)` to
  cover the entire SDK error surface (including direct-factory
  validation, encoder-side failures, runtime/transport, and decoder
  errors). `on AgUiError` still scopes to runtime/transport/decoding
  as before. Added an "Errors" section to the README documenting the
  recommended catch recipe.
- `AGUIValidationError` gained an optional `cause` parameter so the
  `transform`-rethrow path in `JsonDecoder` can preserve structured
  error info instead of flattening to `'Failed to transform field: $e'`.
- `SseParser` documented its per-connection state semantics (sticky
  `_lastEventId`); a new `reset()` method clears all parser state for
  callers that explicitly want to reuse an instance across independent
  streams.
- `Validators.maxTimeout` exposed as `static const Duration` so callers
  can introspect the limit (10 minutes). The cap value is unchanged;
  raising it is deferred to a future release.
- `RunAgentInput.fromJson` and `Run.fromJson` migrated to
  `JsonDecoder.requireEitherField` for consistency with every other
  factory in the SDK. Behavior preserved; the
  "Missing required field 'X' (or 'Y')" wording shifts slightly to match
  the helper's standard error message.
- Long `@Deprecated` messages on the `THINKING_*` enum values and event
  classes hoisted into top-level `const` strings (`event_type.dart`,
  `events.dart`). Surfaces the planned-removal version in one place per
  context and reduces drift risk if it ever changes. No behavior change.

### Documentation
- `UserMessage` documented as a known parity gap with the canonical
  multimodal schema (TS `Union[string, InputContent[]]`, Python
  `Union[str, List[InputContent]]`); the Dart SDK currently only
  supports the string variant.
- `Message.id` documented as nullable-by-type but required-by-convention
  (every concrete subtype constructor declares it `required`); a future
  major version may tighten the type to non-nullable for parity with
  canonical `BaseMessageSchema.id: z.string()`.
- `EventDecoder.validate`'s `Thinking*` deprecated cases gained
  comments explaining why they don't validate `messageId` (the
  deprecated wire shape has no such field; the migration target
  `REASONING_*` does).
- `EventDecoder.validate`'s `ActivityDeltaEvent` case gained a comment
  noting that an empty `patch` is intentional per the canonical
  TS/Python schemas (`z.array(...).min(0)` / list with no length floor).
- `BaseEvent.rawEvent` field gained a dartdoc note clarifying that the
  field is unvalidated (typed `dynamic` because the protocol does not
  constrain the shape).
- `ToolCallResultEvent.role`, `StateSnapshotEvent.snapshot`, and
  `RunErrorEvent.code` field declarations gained a dartdoc note that
  `copyWith(field: null)` does NOT clear the field (these three are the
  remaining cases listed in "Known parity gaps"). Construct a new
  instance directly to drop.
- `MessageRole.activity` and `MessageRole.reasoning` enum values gained
  wire-spelling-pinning dartdoc, mirroring the
  `ReasoningEncryptedValueSubtype.toolCall` style.
- `EventDecoder.validate`'s `ThinkingTextMessageContentEvent` case gained
  a clarified rationale comment: the deprecated path keeps the pre-0.2.0
  stricter "non-empty `delta`" contract intentionally — sibling content
  events (`TextMessageContentEvent`, `ToolCallArgsEvent`,
  `ToolCallResultEvent`, `ReasoningMessageContentEvent`) were RELAXED
  to accept empty strings in 0.2.0 for canonical TS/Python parity, but
  loosening a deprecated contract retroactively serves no one.
- `ReasoningEncryptedValueEvent.fromJson` empty-string rejection comment
  updated to reflect the post-0.2.0 sibling state — it is intentionally
  stricter than the relaxed sibling content events because cipher
  payloads have no defensible "empty" semantic.
- `BaseEvent.fromJson` and `Message.fromJson` switches gained an explicit
  trailing comment stating the analyzer-enforced exhaustiveness so future
  contributors don't add a `default` clause "to be safe."
- `EventStreamAdapter` adopted an internal `_appendDataLine` /
  `flushDataBlock` decomposition to share the per-line and `onDone`
  flush paths in `fromRawSseStream`. No behavior change.
- README "Migrating from 0.1.0" `TimeoutError` → `AGUITimeoutError`
  section gained a paragraph clarifying the symmetric case: consumers
  who previously meant `dart:async.TimeoutError` and were accidentally
  catching SDK instances will see different runtime behavior after they
  fix the import.

## [0.2.0] - 2026-04-30

### Breaking Changes
- `ToolCallResultEvent.role` is now typed `ToolCallResultRole?` instead of
  `String?`. Callers constructing the event directly must use the enum
  (e.g. `ToolCallResultRole.tool`) instead of a raw string. Wire decoding
  is unaffected: an unknown role string on the wire is absorbed via
  `ToolCallResultRole.fromString` and falls back to `ToolCallResultRole.tool`
  (forward-compatible with future canonical roles). The new `role` enum
  exists for parity with the Python `Literal["tool"]` / TypeScript
  `z.literal("tool")` canonical role surface.

### Added
- Activity events for event-type parity with the Python and TypeScript SDKs
  ([#1018](https://github.com/ag-ui-protocol/ag-ui/issues/1018)):
  - `ActivitySnapshotEvent` (`ACTIVITY_SNAPSHOT`)
  - `ActivityDeltaEvent` (`ACTIVITY_DELTA`)
- Reasoning events for event-type parity:
  - `ReasoningStartEvent` (`REASONING_START`)
  - `ReasoningMessageStartEvent` (`REASONING_MESSAGE_START`)
  - `ReasoningMessageContentEvent` (`REASONING_MESSAGE_CONTENT`)
  - `ReasoningMessageEndEvent` (`REASONING_MESSAGE_END`)
  - `ReasoningMessageChunkEvent` (`REASONING_MESSAGE_CHUNK`)
  - `ReasoningEndEvent` (`REASONING_END`)
  - `ReasoningEncryptedValueEvent` (`REASONING_ENCRYPTED_VALUE`)
- Supporting enums: `ReasoningMessageRole`, `ReasoningEncryptedValueSubtype`.
- `ActivityMessage` and `ReasoningMessage` `Message` subtypes (with
  `MessageRole.activity` / `MessageRole.reasoning`) so `MESSAGES_SNAPSHOT`
  payloads carrying those roles decode in Dart with the same schema as the
  canonical TypeScript and Python SDKs. The `activityType` /
  `activity_type` and `encryptedValue` / `encrypted_value` keys both
  decode for camelCase/snake_case parity with the wider protocol.
- Field-level parity for canonical events that previously dropped wire data
  on decode: `TextMessageStartEvent.name`, `TextMessageChunkEvent.name`,
  `RunStartedEvent.parentRunId`, and `RunStartedEvent.input` are now decoded
  and re-emitted by `toJson` so a Dart proxy preserves upstream metadata.
- All event `fromJson` factories now accept both camelCase (TypeScript
  server) and snake_case (Python server) field keys, including the
  pre-existing `TextMessage*` and `ToolCall*` events that were previously
  camelCase-only.
- Decoder-boundary non-empty validation extended to `ToolCallArgsEvent`,
  `ToolCallEndEvent`, `ToolCallResultEvent`, `RunFinishedEvent`,
  `StepStartedEvent`, `StepFinishedEvent`, `StateSnapshotEvent`, `RawEvent`,
  and `CustomEvent` so wire payloads with empty required identifiers or
  missing required content fail at `EventDecoder.decodeJson` instead of
  reaching consumer code as a null/empty value.

### Changed
- `REASONING_MESSAGE_START.role` is now required during decoding to match
  the canonical TypeScript and Python schemas. A payload missing `role`
  now raises `AGUIValidationError` (wrapped as `DecodingError` through
  `EventDecoder`); an unknown role string still falls back to
  `ReasoningMessageRole.reasoning` for forward-compatibility.
- `TextMessageRole.fromString` now throws `ArgumentError` on unknown
  values, mirroring `ReasoningMessageRole.fromString`. Wire decoding is
  unaffected: `TextMessageStartEvent.fromJson` and
  `TextMessageChunkEvent.fromJson` absorb the throw and fall back to
  `TextMessageRole.assistant` for forward compatibility — only direct
  callers of `TextMessageRole.fromString` see the new visible failure
  mode.
- `ReasoningEncryptedValueEvent.fromJson` now wraps an unknown `subtype`
  as `AGUIValidationError` (matching the class-level dartdoc contract),
  instead of leaking the raw `ArgumentError` from
  `ReasoningEncryptedValueSubtype.fromString`. The `EventDecoder`
  pipeline still surfaces it as `DecodingError`.
- `ActivitySnapshotEvent.copyWith` (`content`), `RawEvent.copyWith`
  (`event`), `CustomEvent.copyWith` (`value`), and
  `RunFinishedEvent.copyWith` (`result`) now use an internal sentinel
  parameter so callers can intentionally clear the field to `null`
  (matching each factory contract that already accepted explicit-null
  payloads). Other `copyWith` methods retain the standard
  `?? this.field` pattern (see Known parity gaps).
- `EventDecoder.decodeJson` now wraps `AGUIValidationError` (thrown by
  `fromJson` factories) explicitly so the resulting `DecodingError`
  preserves the original failing field — `role`, `messageId`,
  `subtype`, etc. — instead of flattening to `field: 'json'`. Pre-fix,
  the wrapper relied on the `AgUiError`-based catch path, which
  `AGUIValidationError` (which only `implements Exception`) bypassed.
- `EventDecoder.validate` now rejects an empty `messageId` on
  `TextMessageEndEvent`, restoring symmetry with `TextMessageStartEvent`
  and `TextMessageContentEvent` (and the new reasoning-end events).

### Deprecated
- `EventType.thinkingContent` and `ThinkingContentEvent` — not part of the
  canonical AG-UI protocol. Use `EventType.thinkingTextMessageContent` /
  `ThinkingTextMessageContentEvent` instead. Decoding remains supported for
  backward compatibility; scheduled for removal in 1.0.0.
- `EventType.thinkingTextMessageStart` /
  `EventType.thinkingTextMessageContent` /
  `EventType.thinkingTextMessageEnd` (and their event classes:
  `ThinkingTextMessageStartEvent`, `ThinkingTextMessageContentEvent`,
  `ThinkingTextMessageEndEvent`). Mirrors the canonical TypeScript SDK's
  deprecation of `THINKING_TEXT_MESSAGE_*` in favor of `REASONING_*`. Use
  `ReasoningMessageStartEvent` / `ReasoningMessageContentEvent` /
  `ReasoningMessageEndEvent` instead. Decoding remains supported for
  backward compatibility; scheduled for removal in 1.0.0.

### Known parity gaps (follow-up)
- `copyWith` on some event types with nullable payload fields still uses
  the standard `?? this.field` pattern, which cannot distinguish "omitted"
  from "set to null" — passing `copyWith(field: null)` keeps the existing
  value. The sentinel pattern is now in place for
  `ActivitySnapshotEvent.content`, `RawEvent.event`, `CustomEvent.value`,
  `RunFinishedEvent.result`, the optional fields of
  `TextMessageStartEvent` / `TextMessageChunkEvent`,
  `ToolCallStartEvent.parentMessageId`, the optional fields of
  `ToolCallChunkEvent` and `ReasoningMessageChunkEvent`, and
  `RunStartedEvent.parentRunId` / `RunStartedEvent.input`. The remaining
  `?? this.field` cases are `ToolCallResultEvent.role`,
  `StateSnapshotEvent.snapshot`, and `RunErrorEvent.code`. A sweep across
  these is planned for a future release.

## [0.1.0] - 2025-01-21

### Added
- Initial release of the AG-UI Dart SDK
- Core protocol implementation with full event type support
- HTTP client with Server-Sent Events (SSE) streaming
- Strongly-typed models for all AG-UI protocol entities
- Support for tool interactions and generative UI
- State management with snapshots and JSON Patch deltas (RFC 6902)
- Message history tracking across multiple runs
- Comprehensive error handling with typed exceptions
- Cancel token support for aborting long-running operations
- Environment variable configuration support
- Example CLI application demonstrating key features
- Integration tests validating protocol compliance

### Features
- `AgUiClient` - Main client for AG-UI server interactions
- `SimpleRunAgentInput` - Simplified input structure for common use cases
- Event streaming with backpressure handling
- Tool call processing and result handling
- State synchronization across agent runs
- Message accumulation and conversation context

### Known Limitations
- WebSocket transport not yet implemented
- Binary protocol encoding/decoding not yet supported
- Advanced retry strategies planned for future release
- Event caching and offline support planned for future release

[0.2.0]: https://github.com/ag-ui-protocol/ag-ui/releases/tag/dart-v0.2.0
[0.1.0]: https://github.com/ag-ui-protocol/ag-ui/releases/tag/dart-v0.1.0
