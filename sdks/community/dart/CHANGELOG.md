# Changelog

All notable changes to the AG-UI Dart SDK will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-04-30

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
