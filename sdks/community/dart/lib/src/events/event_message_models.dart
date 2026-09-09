part of 'events.dart';

/// Text message roles that can be used in text message events.
///
/// **Role-fallback convention.** Wire-decoding factories that reference this
/// enum follow a consistent pattern: the enum's `fromString` throws
/// [ArgumentError] for unknown values, and the factory that calls it catches
/// the error and falls back to the canonical role for that event type (e.g.
/// `assistant` for [TextMessageStartEvent], `tool` for
/// [ToolCallResultEvent], `reasoning` for [ReasoningMessageStartEvent]).
/// The one exception is [TextMessageChunkEvent], where `role` is nullable —
/// it falls back to `null` because "present but unrecognized" is distinct
/// from "absent". If you add a new role value here or a new event type that
/// references this enum, update the corresponding factory fall-back as well.
enum TextMessageRole {
  developer('developer'),
  system('system'),
  assistant('assistant'),
  user('user');

  final String value;
  const TextMessageRole(this.value);

  /// Parses [value] into a [TextMessageRole].
  ///
  /// Throws [ArgumentError] for unknown values. Callers decoding from the
  /// wire should use `TextMessageStartEvent.fromJson`, which absorbs the
  /// throw and falls back to [TextMessageRole.assistant] so a future
  /// server-side role does not tear down the SSE stream. This is the
  /// same "throw at the enum, absorb at the factory" pattern used by
  /// [ReasoningMessageRole] — see `dart-enum-parsing-safety.md` for the
  /// consistency rationale.
  static final Map<String, TextMessageRole> _byValue = Map.unmodifiable({
    for (final r in TextMessageRole.values) r.value: r,
  });

  static TextMessageRole fromString(String value) {
    return _byValue[value] ??
        (throw ArgumentError('Invalid text message role: $value'));
  }
}

// ============================================================================
// Text Message Events
// ============================================================================

/// Event indicating the start of a text message
final class TextMessageStartEvent extends _SubagentAttributedEvent {
  final String messageId;
  final TextMessageRole role;
  final String? name;

  const TextMessageStartEvent({
    required this.messageId,
    this.role = TextMessageRole.assistant,
    this.name,
    super.timestamp,
    super.metadata,
    super.subagentRunId,
    super.rawEvent,
  }) : super(eventType: EventType.textMessageStart);

  factory TextMessageStartEvent.fromJson(Map<String, dynamic> json) {
    final messageId = JsonDecoder.requireEitherField<String>(
      json,
      'messageId',
      'message_id',
    );
    final roleStr = JsonDecoder.optionalField<String>(json, 'role');
    var role = TextMessageRole.assistant;
    if (roleStr != null) {
      try {
        role = TextMessageRole.fromString(roleStr);
      } on ArgumentError {
        // Forward-compat: an unknown wire role falls back to
        // `assistant` to keep the stream alive.
        //
        // We intentionally do NOT broaden to `catch (e)` or
        // `on Exception`: a wrong-typed `role` raises
        // `AGUIValidationError` from `optionalField<String>` above, and
        // a missing `messageId` raises `AGUIValidationError` from
        // `requireEitherField` — those MUST propagate to the decoder
        // boundary as protocol violations. Widening the catch would
        // silently absorb them. Mirrors
        // `ReasoningMessageStartEvent.fromJson`.
        role = TextMessageRole.assistant;
      }
    }
    return TextMessageStartEvent(
      metadata: _readMetadata(json),
      subagentRunId: _readSubagentRunId(json),
      messageId: messageId,
      role: role,
      name: JsonDecoder.optionalField<String>(json, 'name'),
      timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
      rawEvent: _readRawEvent(json),
    );
  }

  @override
  Map<String, dynamic> toJson() => {
        ...super.toJson(),
        'messageId': messageId,
        'role': role.value,
        if (name != null) 'name': name,
      };

  // See `_Unset` (top of file) for the sentinel rationale.
  @override
  TextMessageStartEvent copyWith({
    Object? metadata = kUnsetSentinel,
    Object? subagentRunId = kUnsetSentinel,
    String? messageId,
    TextMessageRole? role,
    Object? name = kUnsetSentinel,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return TextMessageStartEvent(
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      subagentRunId: resolveNullableCopy<String>(
        subagentRunId,
        this.subagentRunId,
      ),
      messageId: messageId ?? this.messageId,
      role: role ?? this.role,
      name: identical(name, kUnsetSentinel) ? this.name : name as String?,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event containing text message content
final class TextMessageContentEvent extends _SubagentAttributedEvent {
  final String messageId;
  final String delta;

  const TextMessageContentEvent({
    required this.messageId,
    required this.delta,
    super.timestamp,
    super.metadata,
    super.subagentRunId,
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
    // Empty `delta` is accepted to match canonical TS/Python schemas
    // (`TextMessageContentEventSchema.delta: z.string()` /
    // pydantic `delta: str`). Servers may legitimately emit empty
    // chunks (e.g. a noop content refresh).
    final delta = JsonDecoder.requireField<String>(json, 'delta');

    return TextMessageContentEvent(
      metadata: _readMetadata(json),
      subagentRunId: _readSubagentRunId(json),
      messageId: messageId,
      delta: delta,
      timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
      rawEvent: _readRawEvent(json),
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
    Object? metadata = kUnsetSentinel,
    Object? subagentRunId = kUnsetSentinel,
    String? messageId,
    String? delta,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return TextMessageContentEvent(
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      subagentRunId: resolveNullableCopy<String>(
        subagentRunId,
        this.subagentRunId,
      ),
      messageId: messageId ?? this.messageId,
      delta: delta ?? this.delta,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event indicating the end of a text message
final class TextMessageEndEvent extends _SubagentAttributedEvent {
  final String messageId;

  const TextMessageEndEvent({
    required this.messageId,
    super.timestamp,
    super.metadata,
    super.subagentRunId,
    super.rawEvent,
  }) : super(eventType: EventType.textMessageEnd);

  factory TextMessageEndEvent.fromJson(Map<String, dynamic> json) {
    return TextMessageEndEvent(
      metadata: _readMetadata(json),
      subagentRunId: _readSubagentRunId(json),
      messageId: JsonDecoder.requireEitherField<String>(
        json,
        'messageId',
        'message_id',
      ),
      timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
      rawEvent: _readRawEvent(json),
    );
  }

  @override
  Map<String, dynamic> toJson() => {
        ...super.toJson(),
        'messageId': messageId,
      };

  @override
  TextMessageEndEvent copyWith({
    Object? metadata = kUnsetSentinel,
    Object? subagentRunId = kUnsetSentinel,
    String? messageId,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return TextMessageEndEvent(
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      subagentRunId: resolveNullableCopy<String>(
        subagentRunId,
        this.subagentRunId,
      ),
      messageId: messageId ?? this.messageId,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event containing a chunk of text message content
final class TextMessageChunkEvent extends _SubagentAttributedEvent {
  final String? messageId;
  final TextMessageRole? role;
  final String? delta;
  final String? name;

  const TextMessageChunkEvent({
    this.messageId,
    this.role,
    this.delta,
    this.name,
    super.timestamp,
    super.metadata,
    super.subagentRunId,
    super.rawEvent,
  }) : super(eventType: EventType.textMessageChunk);

  factory TextMessageChunkEvent.fromJson(Map<String, dynamic> json) {
    final roleStr = JsonDecoder.optionalField<String>(json, 'role');
    TextMessageRole? role;
    if (roleStr != null) {
      try {
        role = TextMessageRole.fromString(roleStr);
      } on ArgumentError {
        // Forward-compat: unknown wire role falls back to null.
        // Unlike TextMessageStartEvent (required role → assistant default),
        // role here is nullable/optional — null is the correct sentinel for
        // "value was present on the wire but unrecognized."
        role = null;
      }
    }
    return TextMessageChunkEvent(
      metadata: _readMetadata(json),
      subagentRunId: _readSubagentRunId(json),
      messageId: JsonDecoder.optionalEitherField<String>(
        json,
        'messageId',
        'message_id',
      ),
      role: role,
      delta: JsonDecoder.optionalField<String>(json, 'delta'),
      name: JsonDecoder.optionalField<String>(json, 'name'),
      timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
      rawEvent: _readRawEvent(json),
    );
  }

  @override
  Map<String, dynamic> toJson() => {
        ...super.toJson(),
        if (messageId != null) 'messageId': messageId,
        if (role != null) 'role': role!.value,
        if (delta != null) 'delta': delta,
        if (name != null) 'name': name,
      };

  // See `_Unset` (top of file) for the sentinel rationale.
  @override
  TextMessageChunkEvent copyWith({
    Object? metadata = kUnsetSentinel,
    Object? subagentRunId = kUnsetSentinel,
    Object? messageId = kUnsetSentinel,
    Object? role = kUnsetSentinel,
    Object? delta = kUnsetSentinel,
    Object? name = kUnsetSentinel,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return TextMessageChunkEvent(
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      subagentRunId: resolveNullableCopy<String>(
        subagentRunId,
        this.subagentRunId,
      ),
      messageId: identical(messageId, kUnsetSentinel)
          ? this.messageId
          : messageId as String?,
      role: identical(role, kUnsetSentinel)
          ? this.role
          : role as TextMessageRole?,
      delta: identical(delta, kUnsetSentinel) ? this.delta : delta as String?,
      name: identical(name, kUnsetSentinel) ? this.name : name as String?,
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
    super.metadata,
    super.rawEvent,
  }) : super(eventType: EventType.thinkingStart);

  factory ThinkingStartEvent.fromJson(Map<String, dynamic> json) {
    return ThinkingStartEvent(
      metadata: _readMetadata(json),
      title: JsonDecoder.optionalField<String>(json, 'title'),
      timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
      rawEvent: _readRawEvent(json),
    );
  }

  @override
  Map<String, dynamic> toJson() => {
        ...super.toJson(),
        if (title != null) 'title': title,
      };

  @override
  ThinkingStartEvent copyWith({
    Object? metadata = kUnsetSentinel,
    Object? title = kUnsetSentinel,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ThinkingStartEvent(
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      title: identical(title, kUnsetSentinel) ? this.title : title as String?,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event containing thinking content.
///
/// Dart-only legacy: never part of the canonical AG-UI protocol
/// (TypeScript/Python). Included only for backward compatibility with
/// pre-0.2.0 Dart consumers. Use [ThinkingTextMessageContentEvent] instead.
@Deprecated(_kThinkingContentEventDeprecation)
final class ThinkingContentEvent extends BaseEvent {
  final String delta;

  @Deprecated(_kThinkingContentEventDeprecation)
  const ThinkingContentEvent({
    required this.delta,
    super.timestamp,
    super.metadata,
    super.rawEvent,
  }) : super(eventType: EventType.thinkingContent);

  factory ThinkingContentEvent.fromJson(Map<String, dynamic> json) {
    // Empty `delta` is accepted to match the relaxed canonical contract
    // (`z.string()` / `delta: str`). Migrate to [ReasoningMessageContentEvent].
    final delta = JsonDecoder.requireField<String>(json, 'delta');
    return ThinkingContentEvent(
      metadata: _readMetadata(json),
      delta: delta,
      timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
      rawEvent: _readRawEvent(json),
    );
  }

  @override
  Map<String, dynamic> toJson() => {...super.toJson(), 'delta': delta};

  @override
  ThinkingContentEvent copyWith({
    Object? metadata = kUnsetSentinel,
    String? delta,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ThinkingContentEvent(
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      delta: delta ?? this.delta,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event indicating the end of a thinking section
final class ThinkingEndEvent extends BaseEvent {
  const ThinkingEndEvent({super.timestamp, super.metadata, super.rawEvent})
      : super(eventType: EventType.thinkingEnd);

  factory ThinkingEndEvent.fromJson(Map<String, dynamic> json) {
    return ThinkingEndEvent(
      metadata: _readMetadata(json),
      timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
      rawEvent: _readRawEvent(json),
    );
  }

  @override
  ThinkingEndEvent copyWith({
    Object? metadata = kUnsetSentinel,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ThinkingEndEvent(
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event indicating the start of a thinking text message.
///
/// Deprecated in favor of [ReasoningMessageStartEvent], mirroring the
/// canonical TypeScript SDK deprecation of `THINKING_TEXT_MESSAGE_*` in
/// favor of `REASONING_*`. Decoding remains supported for backward
/// compatibility; scheduled for removal in 1.0.0.
@Deprecated(_kThinkingTextMessageStartEventDeprecation)
final class ThinkingTextMessageStartEvent extends BaseEvent {
  @Deprecated(_kThinkingTextMessageStartEventDeprecation)
  const ThinkingTextMessageStartEvent({
    super.timestamp,
    super.metadata,
    super.rawEvent,
    // ignore: deprecated_member_use_from_same_package
  }) : super(eventType: EventType.thinkingTextMessageStart);

  factory ThinkingTextMessageStartEvent.fromJson(Map<String, dynamic> json) {
    return ThinkingTextMessageStartEvent(
      metadata: _readMetadata(json),
      timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
      rawEvent: _readRawEvent(json),
    );
  }

  @override
  ThinkingTextMessageStartEvent copyWith({
    Object? metadata = kUnsetSentinel,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ThinkingTextMessageStartEvent(
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event containing thinking text message content.
///
/// Deprecated in favor of [ReasoningMessageContentEvent], mirroring the
/// canonical TypeScript SDK deprecation of `THINKING_TEXT_MESSAGE_*` in
/// favor of `REASONING_*`. Decoding remains supported for backward
/// compatibility; scheduled for removal in 1.0.0.
@Deprecated(_kThinkingTextMessageContentEventDeprecation)
final class ThinkingTextMessageContentEvent extends BaseEvent {
  final String delta;

  @Deprecated(_kThinkingTextMessageContentEventDeprecation)
  const ThinkingTextMessageContentEvent({
    required this.delta,
    super.timestamp,
    super.metadata,
    super.rawEvent,
    // ignore: deprecated_member_use_from_same_package
  }) : super(eventType: EventType.thinkingTextMessageContent);

  factory ThinkingTextMessageContentEvent.fromJson(Map<String, dynamic> json) {
    // No identifier on this event. Empty `delta` is accepted to match the
    // relaxed canonical contract (`z.string()` / `delta: str`).
    final delta = JsonDecoder.requireField<String>(json, 'delta');
    return ThinkingTextMessageContentEvent(
      metadata: _readMetadata(json),
      delta: delta,
      timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
      rawEvent: _readRawEvent(json),
    );
  }

  @override
  Map<String, dynamic> toJson() => {...super.toJson(), 'delta': delta};

  @override
  ThinkingTextMessageContentEvent copyWith({
    Object? metadata = kUnsetSentinel,
    String? delta,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ThinkingTextMessageContentEvent(
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      delta: delta ?? this.delta,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event indicating the end of a thinking text message.
///
/// Deprecated in favor of [ReasoningMessageEndEvent], mirroring the
/// canonical TypeScript SDK deprecation of `THINKING_TEXT_MESSAGE_*` in
/// favor of `REASONING_*`. Decoding remains supported for backward
/// compatibility; scheduled for removal in 1.0.0.
@Deprecated(_kThinkingTextMessageEndEventDeprecation)
final class ThinkingTextMessageEndEvent extends BaseEvent {
  @Deprecated(_kThinkingTextMessageEndEventDeprecation)
  const ThinkingTextMessageEndEvent({
    super.timestamp,
    super.metadata,
    super.rawEvent,
    // ignore: deprecated_member_use_from_same_package
  }) : super(eventType: EventType.thinkingTextMessageEnd);

  factory ThinkingTextMessageEndEvent.fromJson(Map<String, dynamic> json) {
    return ThinkingTextMessageEndEvent(
      metadata: _readMetadata(json),
      timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
      rawEvent: _readRawEvent(json),
    );
  }

  @override
  ThinkingTextMessageEndEvent copyWith({
    Object? metadata = kUnsetSentinel,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ThinkingTextMessageEndEvent(
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}
