/// All event types for AG-UI protocol.
///
/// This library defines all event types used in the AG-UI protocol for
/// streaming agent responses and state updates.
///
/// Note: All event classes are in a single file because Dart's sealed classes
/// can only be extended within the same library.
library;

import '../types/base.dart';
import '../types/message.dart';
import '../types/context.dart';
import 'event_type.dart';

export 'event_type.dart';

/// Base event for all AG-UI protocol events.
///
/// All protocol events extend this class and are identified by their
/// [eventType]. Use the [BaseEvent.fromJson] factory to deserialize
/// events from JSON.
sealed class BaseEvent extends AGUIModel with TypeDiscriminator {
  final EventType eventType;
  final int? timestamp;
  final dynamic rawEvent;

  const BaseEvent({
    required this.eventType,
    this.timestamp,
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
  /// Throws [AGUIValidationError] for missing/wrong-typed `type` AND for
  /// unknown event types — `EventType.fromString` raises a raw
  /// `ArgumentError` for unknown values, and we wrap it here so direct
  /// callers see the same error surface as every other validation failure.
  /// (Through the [EventDecoder] pipeline, both surface as [DecodingError].)
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
      case EventType.thinkingTextMessageStart:
        return ThinkingTextMessageStartEvent.fromJson(json);
      case EventType.thinkingTextMessageContent:
        return ThinkingTextMessageContentEvent.fromJson(json);
      case EventType.thinkingTextMessageEnd:
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
    }
  }

  @override
  Map<String, dynamic> toJson() => {
    'type': eventType.value,
    if (timestamp != null) 'timestamp': timestamp,
    if (rawEvent != null) 'rawEvent': rawEvent,
  };
}

/// Text message roles that can be used in text message events.
///
/// Defines the possible roles for text messages in the protocol.
enum TextMessageRole {
  developer('developer'),
  system('system'),
  assistant('assistant'),
  user('user');

  final String value;
  const TextMessageRole(this.value);

  /// Parses [value] into a [TextMessageRole].
  ///
  /// Falls back to [TextMessageRole.assistant] for unknown values to keep
  /// streaming pipelines working when a server adds a new role.
  ///
  /// **Known asymmetry** (tracked for the next major release): other
  /// AG-UI Dart enums (`ReasoningMessageRole`,
  /// `ReasoningEncryptedValueSubtype`) throw on unknown input. The
  /// `ReasoningMessageStartEvent.fromJson` factory absorbs the throw and
  /// falls back to a sane default — that is the "throw at the enum,
  /// absorb at the factory" pattern preferred for new code, because it
  /// keeps the failure mode visible to callers that bypass the factory
  /// and gives the SDK one place to log unknown wire values.
  ///
  /// This silent-fallback in `TextMessageRole.fromString` is historical
  /// and left in place for backward compatibility with existing 0.x
  /// callers. The realignment is documented as a known parity gap in
  /// `CHANGELOG.md` (`[0.2.0]` → "Known parity gaps") and will land with
  /// the 1.0 release.
  static TextMessageRole fromString(String value) {
    return TextMessageRole.values.firstWhere(
      (role) => role.value == value,
      orElse: () => TextMessageRole.assistant,
    );
  }
}

// ============================================================================
// Text Message Events
// ============================================================================

/// Event indicating the start of a text message
final class TextMessageStartEvent extends BaseEvent {
  final String messageId;
  final TextMessageRole role;

  const TextMessageStartEvent({
    required this.messageId,
    this.role = TextMessageRole.assistant,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.textMessageStart);

  factory TextMessageStartEvent.fromJson(Map<String, dynamic> json) {
    return TextMessageStartEvent(
      messageId: JsonDecoder.requireEitherField<String>(
        json,
        'messageId',
        'message_id',
      ),
      role: TextMessageRole.fromString(
        JsonDecoder.optionalField<String>(json, 'role') ?? 'assistant',
      ),
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'messageId': messageId,
    'role': role.value,
  };

  @override
  TextMessageStartEvent copyWith({
    String? messageId,
    TextMessageRole? role,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return TextMessageStartEvent(
      messageId: messageId ?? this.messageId,
      role: role ?? this.role,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event containing text message content
final class TextMessageContentEvent extends BaseEvent {
  final String messageId;
  final String delta;

  const TextMessageContentEvent({
    required this.messageId,
    required this.delta,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.textMessageContent);

  factory TextMessageContentEvent.fromJson(Map<String, dynamic> json) {
    // Validate the cheap required identifier FIRST so a missing-id error
    // surfaces before any payload-validation work — same convention as
    // `ReasoningMessageStartEvent.fromJson`.
    final messageId = JsonDecoder.requireEitherField<String>(
      json,
      'messageId',
      'message_id',
    );
    final delta = JsonDecoder.requireField<String>(json, 'delta');
    if (delta.isEmpty) {
      throw AGUIValidationError(
        message: 'Delta must not be an empty string',
        field: 'delta',
        value: delta,
        json: json,
      );
    }

    return TextMessageContentEvent(
      messageId: messageId,
      delta: delta,
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'messageId': messageId,
    'delta': delta,
  };

  @override
  TextMessageContentEvent copyWith({
    String? messageId,
    String? delta,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return TextMessageContentEvent(
      messageId: messageId ?? this.messageId,
      delta: delta ?? this.delta,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event indicating the end of a text message
final class TextMessageEndEvent extends BaseEvent {
  final String messageId;

  const TextMessageEndEvent({
    required this.messageId,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.textMessageEnd);

  factory TextMessageEndEvent.fromJson(Map<String, dynamic> json) {
    return TextMessageEndEvent(
      messageId: JsonDecoder.requireEitherField<String>(
        json,
        'messageId',
        'message_id',
      ),
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'messageId': messageId,
  };

  @override
  TextMessageEndEvent copyWith({
    String? messageId,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return TextMessageEndEvent(
      messageId: messageId ?? this.messageId,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event containing a chunk of text message content
final class TextMessageChunkEvent extends BaseEvent {
  final String? messageId;
  final TextMessageRole? role;
  final String? delta;

  const TextMessageChunkEvent({
    this.messageId,
    this.role,
    this.delta,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.textMessageChunk);

  factory TextMessageChunkEvent.fromJson(Map<String, dynamic> json) {
    final roleStr = JsonDecoder.optionalField<String>(json, 'role');
    return TextMessageChunkEvent(
      messageId: JsonDecoder.optionalEitherField<String>(
        json,
        'messageId',
        'message_id',
      ),
      role: roleStr != null ? TextMessageRole.fromString(roleStr) : null,
      delta: JsonDecoder.optionalField<String>(json, 'delta'),
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    if (messageId != null) 'messageId': messageId,
    if (role != null) 'role': role!.value,
    if (delta != null) 'delta': delta,
  };

  @override
  TextMessageChunkEvent copyWith({
    String? messageId,
    TextMessageRole? role,
    String? delta,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return TextMessageChunkEvent(
      messageId: messageId ?? this.messageId,
      role: role ?? this.role,
      delta: delta ?? this.delta,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

// ============================================================================
// Thinking Events
// ============================================================================

/// Event indicating the start of a thinking section
final class ThinkingStartEvent extends BaseEvent {
  final String? title;

  const ThinkingStartEvent({
    this.title,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.thinkingStart);

  factory ThinkingStartEvent.fromJson(Map<String, dynamic> json) {
    return ThinkingStartEvent(
      title: JsonDecoder.optionalField<String>(json, 'title'),
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    if (title != null) 'title': title,
  };

  @override
  ThinkingStartEvent copyWith({
    String? title,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ThinkingStartEvent(
      title: title ?? this.title,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event containing thinking content.
///
/// Not part of the canonical AG-UI protocol — included only for
/// backward compatibility. Use [ThinkingTextMessageContentEvent] instead.
@Deprecated(
  'Not part of the canonical AG-UI protocol. '
  'Use ThinkingTextMessageContentEvent instead. '
  'Scheduled for removal in 1.0.0.',
)
final class ThinkingContentEvent extends BaseEvent {
  final String delta;

  const ThinkingContentEvent({
    required this.delta,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.thinkingContent);

  factory ThinkingContentEvent.fromJson(Map<String, dynamic> json) {
    final delta = JsonDecoder.requireField<String>(json, 'delta');
    if (delta.isEmpty) {
      throw AGUIValidationError(
        message: 'Delta must not be an empty string',
        field: 'delta',
        value: delta,
        json: json,
      );
    }
    
    return ThinkingContentEvent(
      delta: delta,
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'delta': delta,
  };

  @override
  ThinkingContentEvent copyWith({
    String? delta,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ThinkingContentEvent(
      delta: delta ?? this.delta,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event indicating the end of a thinking section
final class ThinkingEndEvent extends BaseEvent {
  const ThinkingEndEvent({
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.thinkingEnd);

  factory ThinkingEndEvent.fromJson(Map<String, dynamic> json) {
    return ThinkingEndEvent(
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  ThinkingEndEvent copyWith({
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ThinkingEndEvent(
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event indicating the start of a thinking text message
final class ThinkingTextMessageStartEvent extends BaseEvent {
  const ThinkingTextMessageStartEvent({
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.thinkingTextMessageStart);

  factory ThinkingTextMessageStartEvent.fromJson(Map<String, dynamic> json) {
    return ThinkingTextMessageStartEvent(
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  ThinkingTextMessageStartEvent copyWith({
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ThinkingTextMessageStartEvent(
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event containing thinking text message content
final class ThinkingTextMessageContentEvent extends BaseEvent {
  final String delta;

  const ThinkingTextMessageContentEvent({
    required this.delta,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.thinkingTextMessageContent);

  factory ThinkingTextMessageContentEvent.fromJson(Map<String, dynamic> json) {
    // No identifier on this event — validate the only required payload
    // field. (Comment kept for parity with the sibling `*ContentEvent`
    // factories, which validate `messageId` first.)
    final delta = JsonDecoder.requireField<String>(json, 'delta');
    if (delta.isEmpty) {
      throw AGUIValidationError(
        message: 'Delta must not be an empty string',
        field: 'delta',
        value: delta,
        json: json,
      );
    }

    return ThinkingTextMessageContentEvent(
      delta: delta,
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'delta': delta,
  };

  @override
  ThinkingTextMessageContentEvent copyWith({
    String? delta,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ThinkingTextMessageContentEvent(
      delta: delta ?? this.delta,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event indicating the end of a thinking text message
final class ThinkingTextMessageEndEvent extends BaseEvent {
  const ThinkingTextMessageEndEvent({
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.thinkingTextMessageEnd);

  factory ThinkingTextMessageEndEvent.fromJson(Map<String, dynamic> json) {
    return ThinkingTextMessageEndEvent(
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  ThinkingTextMessageEndEvent copyWith({
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ThinkingTextMessageEndEvent(
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

// ============================================================================
// Tool Call Events
// ============================================================================

/// Event indicating the start of a tool call
final class ToolCallStartEvent extends BaseEvent {
  final String toolCallId;
  final String toolCallName;
  final String? parentMessageId;

  const ToolCallStartEvent({
    required this.toolCallId,
    required this.toolCallName,
    this.parentMessageId,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.toolCallStart);

  factory ToolCallStartEvent.fromJson(Map<String, dynamic> json) {
    return ToolCallStartEvent(
      toolCallId: JsonDecoder.requireEitherField<String>(
        json,
        'toolCallId',
        'tool_call_id',
      ),
      toolCallName: JsonDecoder.requireEitherField<String>(
        json,
        'toolCallName',
        'tool_call_name',
      ),
      parentMessageId: JsonDecoder.optionalEitherField<String>(
        json,
        'parentMessageId',
        'parent_message_id',
      ),
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'toolCallId': toolCallId,
    'toolCallName': toolCallName,
    if (parentMessageId != null) 'parentMessageId': parentMessageId,
  };

  @override
  ToolCallStartEvent copyWith({
    String? toolCallId,
    String? toolCallName,
    String? parentMessageId,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ToolCallStartEvent(
      toolCallId: toolCallId ?? this.toolCallId,
      toolCallName: toolCallName ?? this.toolCallName,
      parentMessageId: parentMessageId ?? this.parentMessageId,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event containing tool call arguments
final class ToolCallArgsEvent extends BaseEvent {
  final String toolCallId;
  final String delta;

  const ToolCallArgsEvent({
    required this.toolCallId,
    required this.delta,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.toolCallArgs);

  factory ToolCallArgsEvent.fromJson(Map<String, dynamic> json) {
    return ToolCallArgsEvent(
      toolCallId: JsonDecoder.requireEitherField<String>(
        json,
        'toolCallId',
        'tool_call_id',
      ),
      delta: JsonDecoder.requireField<String>(json, 'delta'),
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'toolCallId': toolCallId,
    'delta': delta,
  };

  @override
  ToolCallArgsEvent copyWith({
    String? toolCallId,
    String? delta,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ToolCallArgsEvent(
      toolCallId: toolCallId ?? this.toolCallId,
      delta: delta ?? this.delta,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event indicating the end of a tool call
final class ToolCallEndEvent extends BaseEvent {
  final String toolCallId;

  const ToolCallEndEvent({
    required this.toolCallId,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.toolCallEnd);

  factory ToolCallEndEvent.fromJson(Map<String, dynamic> json) {
    return ToolCallEndEvent(
      toolCallId: JsonDecoder.requireEitherField<String>(
        json,
        'toolCallId',
        'tool_call_id',
      ),
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'toolCallId': toolCallId,
  };

  @override
  ToolCallEndEvent copyWith({
    String? toolCallId,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ToolCallEndEvent(
      toolCallId: toolCallId ?? this.toolCallId,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event containing a chunk of tool call content
final class ToolCallChunkEvent extends BaseEvent {
  final String? toolCallId;
  final String? toolCallName;
  final String? parentMessageId;
  final String? delta;

  const ToolCallChunkEvent({
    this.toolCallId,
    this.toolCallName,
    this.parentMessageId,
    this.delta,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.toolCallChunk);

  factory ToolCallChunkEvent.fromJson(Map<String, dynamic> json) {
    return ToolCallChunkEvent(
      toolCallId: JsonDecoder.optionalEitherField<String>(
        json,
        'toolCallId',
        'tool_call_id',
      ),
      toolCallName: JsonDecoder.optionalEitherField<String>(
        json,
        'toolCallName',
        'tool_call_name',
      ),
      parentMessageId: JsonDecoder.optionalEitherField<String>(
        json,
        'parentMessageId',
        'parent_message_id',
      ),
      delta: JsonDecoder.optionalField<String>(json, 'delta'),
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    if (toolCallId != null) 'toolCallId': toolCallId,
    if (toolCallName != null) 'toolCallName': toolCallName,
    if (parentMessageId != null) 'parentMessageId': parentMessageId,
    if (delta != null) 'delta': delta,
  };

  @override
  ToolCallChunkEvent copyWith({
    String? toolCallId,
    String? toolCallName,
    String? parentMessageId,
    String? delta,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ToolCallChunkEvent(
      toolCallId: toolCallId ?? this.toolCallId,
      toolCallName: toolCallName ?? this.toolCallName,
      parentMessageId: parentMessageId ?? this.parentMessageId,
      delta: delta ?? this.delta,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event containing the result of a tool call
final class ToolCallResultEvent extends BaseEvent {
  final String messageId;
  final String toolCallId;
  final String content;
  final String? role;

  const ToolCallResultEvent({
    required this.messageId,
    required this.toolCallId,
    required this.content,
    this.role,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.toolCallResult);

  factory ToolCallResultEvent.fromJson(Map<String, dynamic> json) {
    return ToolCallResultEvent(
      messageId: JsonDecoder.requireEitherField<String>(
        json,
        'messageId',
        'message_id',
      ),
      toolCallId: JsonDecoder.requireEitherField<String>(
        json,
        'toolCallId',
        'tool_call_id',
      ),
      content: JsonDecoder.requireField<String>(json, 'content'),
      role: JsonDecoder.optionalField<String>(json, 'role'),
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'messageId': messageId,
    'toolCallId': toolCallId,
    'content': content,
    if (role != null) 'role': role,
  };

  @override
  ToolCallResultEvent copyWith({
    String? messageId,
    String? toolCallId,
    String? content,
    String? role,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ToolCallResultEvent(
      messageId: messageId ?? this.messageId,
      toolCallId: toolCallId ?? this.toolCallId,
      content: content ?? this.content,
      role: role ?? this.role,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

// ============================================================================
// State Events
// ============================================================================

/// Event containing a snapshot of the state
final class StateSnapshotEvent extends BaseEvent {
  final State snapshot;

  const StateSnapshotEvent({
    required this.snapshot,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.stateSnapshot);

  factory StateSnapshotEvent.fromJson(Map<String, dynamic> json) {
    // `snapshot` may be any JSON shape (including `null` for an empty
    // state), so we cannot use `requireField<T>` (which rejects null
    // values). The field MUST be present though — its absence is a
    // protocol violation, not "the snapshot is empty". Distinguishing
    // missing-key from explicit-null is the whole point of this check.
    if (!json.containsKey('snapshot')) {
      throw AGUIValidationError(
        message: 'Missing required field',
        field: 'snapshot',
        json: json,
      );
    }
    return StateSnapshotEvent(
      snapshot: json['snapshot'],
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'snapshot': snapshot,
  };

  @override
  StateSnapshotEvent copyWith({
    State? snapshot,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return StateSnapshotEvent(
      snapshot: snapshot ?? this.snapshot,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event containing a delta of the state (JSON Patch RFC 6902)
final class StateDeltaEvent extends BaseEvent {
  final List<dynamic> delta;

  const StateDeltaEvent({
    required this.delta,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.stateDelta);

  factory StateDeltaEvent.fromJson(Map<String, dynamic> json) {
    return StateDeltaEvent(
      delta: JsonDecoder.requireField<List<dynamic>>(json, 'delta'),
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'delta': delta,
  };

  @override
  StateDeltaEvent copyWith({
    List<dynamic>? delta,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return StateDeltaEvent(
      delta: delta ?? this.delta,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event containing a snapshot of messages
final class MessagesSnapshotEvent extends BaseEvent {
  final List<Message> messages;

  const MessagesSnapshotEvent({
    required this.messages,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.messagesSnapshot);

  factory MessagesSnapshotEvent.fromJson(Map<String, dynamic> json) {
    return MessagesSnapshotEvent(
      messages: JsonDecoder.requireListField<Map<String, dynamic>>(
        json,
        'messages',
      ).map((item) => Message.fromJson(item)).toList(),
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'messages': messages.map((m) => m.toJson()).toList(),
  };

  @override
  MessagesSnapshotEvent copyWith({
    List<Message>? messages,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return MessagesSnapshotEvent(
      messages: messages ?? this.messages,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

// ============================================================================
// Activity Events
// ============================================================================

/// Event containing a snapshot of an activity message.
///
/// Note: [content] is typed `Object?` rather than `Map<String, dynamic>`.
/// The canonical TypeScript schema requires a non-null record
/// (`z.record(z.any())`); the Dart SDK is intentionally more permissive on
/// the *value* (allows primitives and `null`) to stay forward-compatible
/// with the Python reference server's `content: Any`. The *key itself*
/// is still required — see the matching note on `StateSnapshotEvent.fromJson`
/// for why we check key-presence rather than `requireField<T>`. Treat any
/// non-record value you encounter as a wire-protocol surprise rather than
/// a contract.
final class ActivitySnapshotEvent extends BaseEvent {
  final String messageId;
  final String activityType;
  final Object? content;

  /// `true` (the default) means this snapshot replaces any prior content
  /// for the same [messageId]; `false` means it merges/extends.
  ///
  /// Optional on the wire (`replace: z.boolean().optional().default(true)`
  /// in TS, `replace: bool = True` in Python). [toJson] emits the field
  /// unconditionally — slightly heavier than the protocol minimum, but
  /// makes the round-trip contract explicit and matches what
  /// `event_test.dart` locks in.
  final bool replace;

  const ActivitySnapshotEvent({
    required this.messageId,
    required this.activityType,
    required this.content,
    this.replace = true,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.activitySnapshot);

  factory ActivitySnapshotEvent.fromJson(Map<String, dynamic> json) {
    // `content` may be any JSON shape (including `null`) but MUST be
    // present — see the matching note on `StateSnapshotEvent.fromJson`
    // for why we check key-presence rather than `requireField<T>`.
    if (!json.containsKey('content')) {
      throw AGUIValidationError(
        message: 'Missing required field',
        field: 'content',
        json: json,
      );
    }
    return ActivitySnapshotEvent(
      messageId: JsonDecoder.requireEitherField<String>(
        json,
        'messageId',
        'message_id',
      ),
      activityType: JsonDecoder.requireEitherField<String>(
        json,
        'activityType',
        'activity_type',
      ),
      content: json['content'],
      replace: JsonDecoder.optionalField<bool>(json, 'replace') ?? true,
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'messageId': messageId,
    'activityType': activityType,
    'content': content,
    'replace': replace,
  };

  @override
  ActivitySnapshotEvent copyWith({
    String? messageId,
    String? activityType,
    Object? content,
    bool? replace,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ActivitySnapshotEvent(
      messageId: messageId ?? this.messageId,
      activityType: activityType ?? this.activityType,
      content: content ?? this.content,
      replace: replace ?? this.replace,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event containing a JSON Patch (RFC 6902) delta for an activity message
final class ActivityDeltaEvent extends BaseEvent {
  final String messageId;
  final String activityType;
  final List<dynamic> patch;

  const ActivityDeltaEvent({
    required this.messageId,
    required this.activityType,
    required this.patch,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.activityDelta);

  factory ActivityDeltaEvent.fromJson(Map<String, dynamic> json) {
    return ActivityDeltaEvent(
      messageId: JsonDecoder.requireEitherField<String>(
        json,
        'messageId',
        'message_id',
      ),
      activityType: JsonDecoder.requireEitherField<String>(
        json,
        'activityType',
        'activity_type',
      ),
      patch: JsonDecoder.requireField<List<dynamic>>(json, 'patch'),
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'messageId': messageId,
    'activityType': activityType,
    'patch': patch,
  };

  @override
  ActivityDeltaEvent copyWith({
    String? messageId,
    String? activityType,
    List<dynamic>? patch,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ActivityDeltaEvent(
      messageId: messageId ?? this.messageId,
      activityType: activityType ?? this.activityType,
      patch: patch ?? this.patch,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event containing a raw event
final class RawEvent extends BaseEvent {
  final dynamic event;
  final String? source;

  const RawEvent({
    required this.event,
    this.source,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.raw);

  factory RawEvent.fromJson(Map<String, dynamic> json) {
    // `event` may be any JSON shape but MUST be present — see the
    // matching note on `StateSnapshotEvent.fromJson` for why we check
    // key-presence rather than `requireField<T>`.
    if (!json.containsKey('event')) {
      throw AGUIValidationError(
        message: 'Missing required field',
        field: 'event',
        json: json,
      );
    }
    return RawEvent(
      event: json['event'],
      source: JsonDecoder.optionalField<String>(json, 'source'),
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'event': event,
    if (source != null) 'source': source,
  };

  @override
  RawEvent copyWith({
    dynamic newEvent,
    String? source,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return RawEvent(
      event: newEvent ?? this.event,
      source: source ?? this.source,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event containing a custom event
final class CustomEvent extends BaseEvent {
  final String name;
  final dynamic value;

  const CustomEvent({
    required this.name,
    required this.value,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.custom);

  factory CustomEvent.fromJson(Map<String, dynamic> json) {
    // `value` may be any JSON shape but MUST be present — see the
    // matching note on `StateSnapshotEvent.fromJson` for why we check
    // key-presence rather than `requireField<T>`.
    if (!json.containsKey('value')) {
      throw AGUIValidationError(
        message: 'Missing required field',
        field: 'value',
        json: json,
      );
    }
    return CustomEvent(
      name: JsonDecoder.requireField<String>(json, 'name'),
      value: json['value'],
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'name': name,
    'value': value,
  };

  @override
  CustomEvent copyWith({
    String? name,
    dynamic value,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return CustomEvent(
      name: name ?? this.name,
      value: value ?? this.value,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

// ============================================================================
// Lifecycle Events
// ============================================================================

/// Event indicating that a run has started
final class RunStartedEvent extends BaseEvent {
  final String threadId;
  final String runId;

  const RunStartedEvent({
    required this.threadId,
    required this.runId,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.runStarted);

  factory RunStartedEvent.fromJson(Map<String, dynamic> json) {
    return RunStartedEvent(
      threadId: JsonDecoder.requireEitherField<String>(
        json,
        'threadId',
        'thread_id',
      ),
      runId: JsonDecoder.requireEitherField<String>(
        json,
        'runId',
        'run_id',
      ),
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'threadId': threadId,
    'runId': runId,
  };

  @override
  RunStartedEvent copyWith({
    String? threadId,
    String? runId,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return RunStartedEvent(
      threadId: threadId ?? this.threadId,
      runId: runId ?? this.runId,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event indicating that a run has finished
final class RunFinishedEvent extends BaseEvent {
  final String threadId;
  final String runId;
  final dynamic result;

  const RunFinishedEvent({
    required this.threadId,
    required this.runId,
    this.result,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.runFinished);

  factory RunFinishedEvent.fromJson(Map<String, dynamic> json) {
    return RunFinishedEvent(
      threadId: JsonDecoder.requireEitherField<String>(
        json,
        'threadId',
        'thread_id',
      ),
      runId: JsonDecoder.requireEitherField<String>(
        json,
        'runId',
        'run_id',
      ),
      result: json['result'],
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'threadId': threadId,
    'runId': runId,
    if (result != null) 'result': result,
  };

  @override
  RunFinishedEvent copyWith({
    String? threadId,
    String? runId,
    dynamic result,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return RunFinishedEvent(
      threadId: threadId ?? this.threadId,
      runId: runId ?? this.runId,
      result: result ?? this.result,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event indicating that a run has encountered an error
final class RunErrorEvent extends BaseEvent {
  final String message;
  final String? code;

  const RunErrorEvent({
    required this.message,
    this.code,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.runError);

  factory RunErrorEvent.fromJson(Map<String, dynamic> json) {
    return RunErrorEvent(
      message: JsonDecoder.requireField<String>(json, 'message'),
      code: JsonDecoder.optionalField<String>(json, 'code'),
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'message': message,
    if (code != null) 'code': code,
  };

  @override
  RunErrorEvent copyWith({
    String? message,
    String? code,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return RunErrorEvent(
      message: message ?? this.message,
      code: code ?? this.code,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event indicating that a step has started
final class StepStartedEvent extends BaseEvent {
  final String stepName;

  const StepStartedEvent({
    required this.stepName,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.stepStarted);

  factory StepStartedEvent.fromJson(Map<String, dynamic> json) {
    return StepStartedEvent(
      stepName: JsonDecoder.requireEitherField<String>(
        json,
        'stepName',
        'step_name',
      ),
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'stepName': stepName,
  };

  @override
  StepStartedEvent copyWith({
    String? stepName,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return StepStartedEvent(
      stepName: stepName ?? this.stepName,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event indicating that a step has finished
final class StepFinishedEvent extends BaseEvent {
  final String stepName;

  const StepFinishedEvent({
    required this.stepName,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.stepFinished);

  factory StepFinishedEvent.fromJson(Map<String, dynamic> json) {
    return StepFinishedEvent(
      stepName: JsonDecoder.requireEitherField<String>(
        json,
        'stepName',
        'step_name',
      ),
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'stepName': stepName,
  };

  @override
  StepFinishedEvent copyWith({
    String? stepName,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return StepFinishedEvent(
      stepName: stepName ?? this.stepName,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

// ============================================================================
// Reasoning Events
// ============================================================================

/// Role for reasoning messages (aligned with the AG-UI protocol).
///
/// Currently a single-variant enum mirroring the canonical
/// `Literal["reasoning"]` (Python) / `z.literal("reasoning")` (TypeScript).
/// Modeled as an enum so a future role addition can land without churning
/// every call site.
enum ReasoningMessageRole {
  reasoning('reasoning');

  final String value;
  const ReasoningMessageRole(this.value);

  /// Parses [value] into a [ReasoningMessageRole].
  ///
  /// Throws [ArgumentError] for unknown values. Callers decoding from the
  /// wire should use `ReasoningMessageStartEvent.fromJson`, which absorbs
  /// the throw and falls back to [ReasoningMessageRole.reasoning] so a
  /// future server-side role does not tear down the SSE stream.
  static ReasoningMessageRole fromString(String value) {
    return ReasoningMessageRole.values.firstWhere(
      (role) => role.value == value,
      orElse: () => throw ArgumentError(
        'Invalid reasoning message role: $value',
      ),
    );
  }
}

/// Subtype for [ReasoningEncryptedValueEvent].
enum ReasoningEncryptedValueSubtype {
  toolCall('tool-call'),
  message('message');

  final String value;
  const ReasoningEncryptedValueSubtype(this.value);

  /// Parses [value] into a [ReasoningEncryptedValueSubtype].
  ///
  /// Throws [ArgumentError] for unknown values. The subtype is part of the
  /// protocol contract — there is no graceful fallback at the event level
  /// because choosing a default would silently mis-tag encrypted payloads.
  /// Wire failures bubble up as [DecodingError] under the standard decoder
  /// pipeline; consumers that want per-event recovery should set
  /// `skipInvalidEvents: true` on `EventStreamAdapter`.
  static ReasoningEncryptedValueSubtype fromString(String value) {
    return ReasoningEncryptedValueSubtype.values.firstWhere(
      (s) => s.value == value,
      orElse: () => throw ArgumentError(
        'Invalid reasoning encrypted value subtype: $value',
      ),
    );
  }
}

/// Event indicating the start of a reasoning phase.
final class ReasoningStartEvent extends BaseEvent {
  final String messageId;

  const ReasoningStartEvent({
    required this.messageId,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.reasoningStart);

  factory ReasoningStartEvent.fromJson(Map<String, dynamic> json) {
    return ReasoningStartEvent(
      messageId: JsonDecoder.requireEitherField<String>(
        json,
        'messageId',
        'message_id',
      ),
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'messageId': messageId,
  };

  @override
  ReasoningStartEvent copyWith({
    String? messageId,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ReasoningStartEvent(
      messageId: messageId ?? this.messageId,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event indicating the start of a reasoning message.
final class ReasoningMessageStartEvent extends BaseEvent {
  final String messageId;
  final ReasoningMessageRole role;

  const ReasoningMessageStartEvent({
    required this.messageId,
    this.role = ReasoningMessageRole.reasoning,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.reasoningMessageStart);

  factory ReasoningMessageStartEvent.fromJson(Map<String, dynamic> json) {
    // Validate the cheap required field FIRST so a missing-id error
    // surfaces before any role-parsing work.
    final messageId = JsonDecoder.requireEitherField<String>(
      json,
      'messageId',
      'message_id',
    );
    // `role` is required by the canonical TypeScript and Python schemas
    // (see sdks/typescript/packages/core/src/events.ts and
    // sdks/python/ag_ui/core/events.py). A missing `role` is a protocol
    // violation and must fail decoding so it surfaces at the boundary
    // instead of silently coercing downstream.
    final roleStr = JsonDecoder.requireField<String>(json, 'role');
    ReasoningMessageRole role;
    try {
      role = ReasoningMessageRole.fromString(roleStr);
    } on ArgumentError {
      // Forward-compat: a future server may introduce a new role *value*
      // (e.g. an as-yet-unspecified reasoning sub-role). The field is
      // present and string-typed, so this is a recoverable enum-mapping
      // failure — keep the stream alive by defaulting to `reasoning`.
      //
      // We intentionally do NOT broaden to `catch (e)` or `on Exception`:
      // a missing-key or wrong-typed `role` raises `AGUIValidationError`
      // from `requireField<String>` above, which MUST propagate to the
      // decoder boundary as a protocol violation. Widening the catch
      // would silently absorb those — the test at
      // `event_test.dart` ("rejects missing role (parity with TS/Python)")
      // is the regression guard for that contract.
      role = ReasoningMessageRole.reasoning;
    }
    return ReasoningMessageStartEvent(
      messageId: messageId,
      role: role,
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'messageId': messageId,
    'role': role.value,
  };

  @override
  ReasoningMessageStartEvent copyWith({
    String? messageId,
    ReasoningMessageRole? role,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ReasoningMessageStartEvent(
      messageId: messageId ?? this.messageId,
      role: role ?? this.role,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event containing a piece of reasoning message content.
final class ReasoningMessageContentEvent extends BaseEvent {
  final String messageId;
  final String delta;

  const ReasoningMessageContentEvent({
    required this.messageId,
    required this.delta,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.reasoningMessageContent);

  factory ReasoningMessageContentEvent.fromJson(Map<String, dynamic> json) {
    // Validate the cheap required identifier FIRST so a missing-id error
    // surfaces before any payload-validation work — same convention as
    // `ReasoningMessageStartEvent.fromJson`.
    final messageId = JsonDecoder.requireEitherField<String>(
      json,
      'messageId',
      'message_id',
    );
    final delta = JsonDecoder.requireField<String>(json, 'delta');
    if (delta.isEmpty) {
      throw AGUIValidationError(
        message: 'Delta must not be an empty string',
        field: 'delta',
        value: delta,
        json: json,
      );
    }

    return ReasoningMessageContentEvent(
      messageId: messageId,
      delta: delta,
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'messageId': messageId,
    'delta': delta,
  };

  @override
  ReasoningMessageContentEvent copyWith({
    String? messageId,
    String? delta,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ReasoningMessageContentEvent(
      messageId: messageId ?? this.messageId,
      delta: delta ?? this.delta,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event indicating the end of a reasoning message.
final class ReasoningMessageEndEvent extends BaseEvent {
  final String messageId;

  const ReasoningMessageEndEvent({
    required this.messageId,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.reasoningMessageEnd);

  factory ReasoningMessageEndEvent.fromJson(Map<String, dynamic> json) {
    return ReasoningMessageEndEvent(
      messageId: JsonDecoder.requireEitherField<String>(
        json,
        'messageId',
        'message_id',
      ),
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'messageId': messageId,
  };

  @override
  ReasoningMessageEndEvent copyWith({
    String? messageId,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ReasoningMessageEndEvent(
      messageId: messageId ?? this.messageId,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event containing a chunk of reasoning message content.
final class ReasoningMessageChunkEvent extends BaseEvent {
  final String? messageId;
  final String? delta;

  const ReasoningMessageChunkEvent({
    this.messageId,
    this.delta,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.reasoningMessageChunk);

  factory ReasoningMessageChunkEvent.fromJson(Map<String, dynamic> json) {
    return ReasoningMessageChunkEvent(
      messageId: JsonDecoder.optionalEitherField<String>(
        json,
        'messageId',
        'message_id',
      ),
      // `delta` has no snake_case spelling in any AG-UI SDK — read it
      // canonically and skip the dual-key lookup.
      delta: JsonDecoder.optionalField<String>(json, 'delta'),
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    if (messageId != null) 'messageId': messageId,
    if (delta != null) 'delta': delta,
  };

  @override
  ReasoningMessageChunkEvent copyWith({
    String? messageId,
    String? delta,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ReasoningMessageChunkEvent(
      messageId: messageId ?? this.messageId,
      delta: delta ?? this.delta,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event indicating the end of a reasoning phase.
final class ReasoningEndEvent extends BaseEvent {
  final String messageId;

  const ReasoningEndEvent({
    required this.messageId,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.reasoningEnd);

  factory ReasoningEndEvent.fromJson(Map<String, dynamic> json) {
    return ReasoningEndEvent(
      messageId: JsonDecoder.requireEitherField<String>(
        json,
        'messageId',
        'message_id',
      ),
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'messageId': messageId,
  };

  @override
  ReasoningEndEvent copyWith({
    String? messageId,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ReasoningEndEvent(
      messageId: messageId ?? this.messageId,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event containing an encrypted value for a message or tool call.
///
/// Forward-compat note: a future server-side [subtype] value will cause
/// [ReasoningEncryptedValueSubtype.fromString] to throw, which propagates
/// out of `fromJson` as an [AGUIValidationError] (wrapped in a
/// [DecodingError] when reached through [EventDecoder]). To keep streams
/// alive across an unknown subtype, opt in to per-event recovery via
/// `EventStreamAdapter(skipInvalidEvents: true)` — the rest of the SDK's
/// enums absorb unknown values at the event-decoding boundary, but the
/// encrypted-payload subtype has no sensible default to fall back to.
final class ReasoningEncryptedValueEvent extends BaseEvent {
  final ReasoningEncryptedValueSubtype subtype;
  final String entityId;
  final String encryptedValue;

  const ReasoningEncryptedValueEvent({
    required this.subtype,
    required this.entityId,
    required this.encryptedValue,
    super.timestamp,
    super.rawEvent,
  }) : super(eventType: EventType.reasoningEncryptedValue);

  factory ReasoningEncryptedValueEvent.fromJson(Map<String, dynamic> json) {
    final subtypeStr = JsonDecoder.requireField<String>(json, 'subtype');
    return ReasoningEncryptedValueEvent(
      subtype: ReasoningEncryptedValueSubtype.fromString(subtypeStr),
      entityId: JsonDecoder.requireEitherField<String>(
        json,
        'entityId',
        'entity_id',
      ),
      encryptedValue: JsonDecoder.requireEitherField<String>(
        json,
        'encryptedValue',
        'encrypted_value',
      ),
      timestamp: JsonDecoder.optionalField<int>(json, 'timestamp'),
      rawEvent: json['rawEvent'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'subtype': subtype.value,
    'entityId': entityId,
    'encryptedValue': encryptedValue,
  };

  @override
  ReasoningEncryptedValueEvent copyWith({
    ReasoningEncryptedValueSubtype? subtype,
    String? entityId,
    String? encryptedValue,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ReasoningEncryptedValueEvent(
      subtype: subtype ?? this.subtype,
      entityId: entityId ?? this.entityId,
      encryptedValue: encryptedValue ?? this.encryptedValue,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}