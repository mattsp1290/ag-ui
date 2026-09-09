/// All event types for AG-UI protocol.
///
/// This library defines all event types used in the AG-UI protocol for
/// streaming agent responses and state updates.
///
/// Event families live in part files so the sealed hierarchy remains in this
/// library while each editing unit stays small.
library;

import 'dart:developer' as developer;

import '../types/base.dart';
import '../types/context.dart';
import '../types/copy_utils.dart';
import '../types/metadata.dart';
import '../types/message.dart';
import '../types/run_outcome.dart';
import '../types/subagent_outcome.dart';
import '../types/wire_safety.dart';
import 'event_type.dart';

export 'event_type.dart';

part 'event_message_models.dart';
part 'event_tool_state_models.dart';
part 'event_activity_models.dart';
part 'event_lifecycle_models.dart';
part 'event_subagent_models.dart';
part 'event_reasoning_models.dart';

// `kUnsetSentinel` (from `base.dart`) is the shared sentinel for all
// `copyWith` methods in this file. With the default `?? this.field` pattern,
// a caller cannot distinguish "argument omitted" from "argument explicitly set
// to `null`". Comparing against `kUnsetSentinel` with `identical(...)` makes
// that distinction explicit.
//
// **`rawEvent` is intentionally sticky** — all `copyWith` methods use
// `rawEvent ?? this.rawEvent` rather than the sentinel pattern. Passing
// `null` for `rawEvent` keeps the existing value; to clear it, construct
// the event directly with `rawEvent: null`. This is a deliberate design:
// `ReasoningEncryptedValueEvent.fromJson` explicitly sets `rawEvent: null`
// to scrub cipher data, and the sentinel approach would inadvertently
// re-expose a prior non-null value when the caller omits the argument.
// See `BaseEvent.rawEvent` dartdoc for the full consumer note.
//
// Applied to every nullable payload field on the events whose `copyWith`
// callers may legitimately want to clear:
// `ActivitySnapshotEvent.content`, `RawEvent.event`, `CustomEvent.value`,
// `RunFinishedEvent.result`, `RunStartedEvent.parentRunId` /
// `RunStartedEvent.input`, the `name` field of `TextMessageStartEvent`,
// the optional fields of `TextMessageChunkEvent`,
// `ToolCallStartEvent.parentMessageId`, the optional fields of
// `ToolCallChunkEvent`, the optional fields of `ReasoningMessageChunkEvent`,
// `ThinkingStartEvent.title`, `ToolCallResultEvent.role`,
// `StateSnapshotEvent.snapshot`, and `RunErrorEvent.code`.

/// Reads the `rawEvent` field from a wire payload, accepting both
/// `rawEvent` (TypeScript-canonical) and `raw_event` (Python-canonical).
/// `containsKey` precedence — a present `rawEvent` key wins even when its
/// value is explicitly `null`, matching the documented `requireEitherField`
/// rule for camelCase-vs-snake_case dual reads. Used by every event
/// factory in this library so a Python-emitted `raw_event` survives the
/// proxy round-trip.
dynamic _readRawEvent(Map<String, dynamic> json) =>
    json.containsKey('rawEvent') ? json['rawEvent'] : json['raw_event'];

/// Reads optional typed metadata from a wire payload.
Metadata? _readMetadata(Map<String, dynamic> json) =>
    readCipherAwareOptionalField<Map<String, dynamic>>(json, 'metadata');

String? _readSubagentRunId(Map<String, dynamic> json) =>
    readCipherAwareOptionalEitherField<String>(
      json,
      'subagentRunId',
      'subagent_run_id',
    );

RunFinishedOutcome? _readRunFinishedOutcome(Map<String, dynamic> json) {
  if (!json.containsKey('outcome') || json['outcome'] == null) {
    return null;
  }
  try {
    final outcome = JsonDecoder.requireField<Map<String, dynamic>>(
      json,
      'outcome',
    );
    return RunFinishedOutcome.fromJson(outcome);
  } on AGUIValidationError catch (error) {
    final nestedField = error.field;
    throw wrapNestedValidationError(
      enclosingJson: json,
      error: error,
      field: nestedField == null || nestedField == 'outcome'
          ? 'outcome'
          : 'outcome.$nestedField',
    );
  }
}

// Hoisted `@Deprecated` messages: each is repeated on the class
// declaration AND the constructor of the corresponding event type, so a
// constant lets the planned-removal version (1.0.0) and migration target
// get edited in one place per event class. Sibling enum-side messages
// live in `event_type.dart`; the surfaces are intentionally different
// (enum names vs. event class names).
// IMPORTANT: Do NOT add `// ignore_for_file: deprecated_member_use_from_same_package`
// to this file. The per-line `// ignore:` comments below are load-bearing:
// they enumerate every deprecated event type use so the 1.0.0 removal sweep
// knows exactly which lines to delete. A file-level suppression would silence
// the deprecation alarm and make the sweep invisible to the analyzer.
const String _kThinkingTextMessageStartEventDeprecation =
    'Use ReasoningMessageStartEvent instead. '
    'Scheduled for removal in 1.0.0.';
const String _kThinkingTextMessageContentEventDeprecation =
    'Use ReasoningMessageContentEvent instead. '
    'Scheduled for removal in 1.0.0.';
const String _kThinkingTextMessageEndEventDeprecation =
    'Use ReasoningMessageEndEvent instead. '
    'Scheduled for removal in 1.0.0.';
const String _kThinkingContentEventDeprecation =
    'Dart-only legacy: never part of the canonical AG-UI protocol '
    '(TypeScript/Python). '
    'Use ReasoningMessageContentEvent instead. '
    'Scheduled for removal in 1.0.0.';

/// Base event for all AG-UI protocol events.
///
/// All protocol events extend this class and are identified by their
/// [eventType]. Use the [BaseEvent.fromJson] factory to deserialize
/// events from JSON.
sealed class BaseEvent extends AGUIModel with TypeDiscriminator {
  final EventType eventType;
  final int? timestamp;
  final Metadata? metadata;

  /// The original wire-format payload, preserved verbatim for proxy
  /// scenarios. Typed `dynamic` because the protocol does not constrain
  /// the shape (TS: `z.unknown()`, Python: `Any`). No validation is
  /// performed; the raw value flows through unchanged via every factory
  /// (which reads both `rawEvent` and `raw_event` via the private
  /// `_readRawEvent` helper, with camelCase precedence) and is
  /// re-emitted as-is from `toJson` when non-null.
  ///
  /// **Consumer note: round-trip emission.** Anything assigned to this
  /// field WILL be serialized on the next `encode`. If you don't want
  /// the upstream payload echoed downstream, set `rawEvent: null` on
  /// the in-flight event before re-encoding by constructing a new event
  /// directly with `rawEvent: null`. Wire output uses the camelCase key
  /// `rawEvent` regardless of which spelling came in.
  ///
  /// **Cipher-safety hazard.** Event types that nest [Message] objects with
  /// `encryptedValue` payloads (currently [MessagesSnapshotEvent] and
  /// [RunStartedEvent]) force `rawEvent` to `null` in their `fromJson` and
  /// `copyWith` implementations, because the verbatim wire map would expose
  /// the cipher payload that the inner message factories intentionally
  /// omitted. Callers building these events in memory (without going through
  /// `fromJson`) are responsible for setting `rawEvent: null` when the
  /// structured payload contains `encryptedValue` data.
  final dynamic rawEvent;

  const BaseEvent({
    required this.eventType,
    this.timestamp,
    this.metadata,
    this.rawEvent,
  });

  @override
  String get type => eventType.value;

  /// Factory constructor to create specific event types from JSON.
  ///
  /// When you add a case here, also update `EventDecoder.validate` in
  /// `lib/src/encoder/decoder.dart` so the analyzer-enforced exhaustive
  /// switch on the sealed `BaseEvent` hierarchy continues to compile.
  ///
  /// **Error surface.** Throws [AGUIValidationError] for:
  /// - Missing or wrong-typed `type` field.
  /// - Unknown event type string (wrapped from `ArgumentError`).
  /// - Any per-event-factory field validation failure (missing required field,
  ///   wrong type, enum parse error, etc.) — these are thrown directly by the
  ///   delegate factory and propagate unchanged.
  ///
  /// Through the [EventDecoder] pipeline all of the above surface as
  /// [DecodingError]. Direct callers that bypass [EventDecoder] should catch
  /// [AGUIValidationError]. Direct callers should also run
  /// `EventDecoder.validate(event)` after this factory if they want
  /// non-empty-field enforcement (e.g. non-null `messageId`) — this factory
  /// only enforces field presence and type, not semantic constraints.
  ///
  /// Note on equality: event subtypes are `final class` and do NOT
  /// override `==`/`hashCode`. Use field-by-field assertions in tests
  /// rather than `expect(a, equals(b))` on whole events.
  factory BaseEvent.fromJson(Map<String, dynamic> json) {
    final typeStr = JsonDecoder.requireField<String>(json, 'type');
    final EventType eventType;
    try {
      eventType = EventType.fromString(typeStr);
    } on ArgumentError {
      throw AGUIValidationError(
        message: 'Unknown event type: $typeStr',
        field: 'type',
        value: typeStr,
        json: json,
      );
    }

    switch (eventType) {
      case EventType.textMessageStart:
        return TextMessageStartEvent.fromJson(json);
      case EventType.textMessageContent:
        return TextMessageContentEvent.fromJson(json);
      case EventType.textMessageEnd:
        return TextMessageEndEvent.fromJson(json);
      case EventType.textMessageChunk:
        return TextMessageChunkEvent.fromJson(json);
      // TODO(1.0.0): Remove the following deprecated cases + their event classes:
      //   ThinkingTextMessageStartEvent, ThinkingTextMessageContentEvent,
      //   ThinkingTextMessageEndEvent, ThinkingContentEvent.
      //   Also remove EventType.thinkingTextMessage* / thinkingContent enum
      //   values, the _kThinkingTextMessage*Deprecation / _kThinkingContent*
      //   Deprecation constants, and the deprecated TimeoutError typedef in
      //   client/errors.dart.
      // ignore: deprecated_member_use_from_same_package
      case EventType.thinkingTextMessageStart:
        // ignore: deprecated_member_use_from_same_package
        return ThinkingTextMessageStartEvent.fromJson(json);
      // ignore: deprecated_member_use_from_same_package
      case EventType.thinkingTextMessageContent:
        // ignore: deprecated_member_use_from_same_package
        return ThinkingTextMessageContentEvent.fromJson(json);
      // ignore: deprecated_member_use_from_same_package
      case EventType.thinkingTextMessageEnd:
        // ignore: deprecated_member_use_from_same_package
        return ThinkingTextMessageEndEvent.fromJson(json);
      case EventType.toolCallStart:
        return ToolCallStartEvent.fromJson(json);
      case EventType.toolCallArgs:
        return ToolCallArgsEvent.fromJson(json);
      case EventType.toolCallEnd:
        return ToolCallEndEvent.fromJson(json);
      case EventType.toolCallChunk:
        return ToolCallChunkEvent.fromJson(json);
      case EventType.toolCallResult:
        return ToolCallResultEvent.fromJson(json);
      case EventType.thinkingStart:
        return ThinkingStartEvent.fromJson(json);
      // ignore: deprecated_member_use_from_same_package
      case EventType.thinkingContent:
        // ignore: deprecated_member_use_from_same_package
        return ThinkingContentEvent.fromJson(json);
      case EventType.thinkingEnd:
        return ThinkingEndEvent.fromJson(json);
      case EventType.stateSnapshot:
        return StateSnapshotEvent.fromJson(json);
      case EventType.stateDelta:
        return StateDeltaEvent.fromJson(json);
      case EventType.messagesSnapshot:
        return MessagesSnapshotEvent.fromJson(json);
      case EventType.activitySnapshot:
        return ActivitySnapshotEvent.fromJson(json);
      case EventType.activityDelta:
        return ActivityDeltaEvent.fromJson(json);
      case EventType.raw:
        return RawEvent.fromJson(json);
      case EventType.custom:
        return CustomEvent.fromJson(json);
      case EventType.runStarted:
        return RunStartedEvent.fromJson(json);
      case EventType.runFinished:
        return RunFinishedEvent.fromJson(json);
      case EventType.runError:
        return RunErrorEvent.fromJson(json);
      case EventType.stepStarted:
        return StepStartedEvent.fromJson(json);
      case EventType.stepFinished:
        return StepFinishedEvent.fromJson(json);
      case EventType.reasoningStart:
        return ReasoningStartEvent.fromJson(json);
      case EventType.reasoningMessageStart:
        return ReasoningMessageStartEvent.fromJson(json);
      case EventType.reasoningMessageContent:
        return ReasoningMessageContentEvent.fromJson(json);
      case EventType.reasoningMessageEnd:
        return ReasoningMessageEndEvent.fromJson(json);
      case EventType.reasoningMessageChunk:
        return ReasoningMessageChunkEvent.fromJson(json);
      case EventType.reasoningEnd:
        return ReasoningEndEvent.fromJson(json);
      case EventType.reasoningEncryptedValue:
        return ReasoningEncryptedValueEvent.fromJson(json);
      case EventType.subagentStarted:
        return SubagentStartedEvent.fromJson(json);
      case EventType.subagentFinished:
        return SubagentFinishedEvent.fromJson(json);
      case EventType.subagentError:
        return SubagentErrorEvent.fromJson(json);
      // No `default` clause — exhaustive switch on the [EventType] enum
      // (analyzer-enforced). A new EventType value will produce a compile
      // error here AND in `EventDecoder.validate`, which is the desired
      // outcome rather than a runtime fall-through.
    }
  }

  @override
  Map<String, dynamic> toJson() => {
        'type': eventType.value,
        if (timestamp != null) 'timestamp': timestamp,
        if (metadata != null) 'metadata': metadata,
        if (rawEvent != null) 'rawEvent': rawEvent,
      };
}

/// Internal base for the exact event set that carries subagent attribution.
sealed class _SubagentAttributedEvent extends BaseEvent {
  final String? subagentRunId;

  const _SubagentAttributedEvent({
    required super.eventType,
    super.timestamp,
    super.metadata,
    this.subagentRunId,
    super.rawEvent,
  });

  @override
  Map<String, dynamic> toJson() => {
        ...super.toJson(),
        if (subagentRunId != null) 'subagentRunId': subagentRunId,
      };
}
