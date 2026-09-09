part of 'events.dart';

// ============================================================================
// Tool Call Events
// ============================================================================

/// Event indicating the start of a tool call
final class ToolCallStartEvent extends _SubagentAttributedEvent {
  final String toolCallId;
  final String toolCallName;
  final String? parentMessageId;

  const ToolCallStartEvent({
    required this.toolCallId,
    required this.toolCallName,
    this.parentMessageId,
    super.timestamp,
    super.metadata,
    super.subagentRunId,
    super.rawEvent,
  }) : super(eventType: EventType.toolCallStart);

  factory ToolCallStartEvent.fromJson(Map<String, dynamic> json) {
    return ToolCallStartEvent(
      metadata: _readMetadata(json),
      subagentRunId: _readSubagentRunId(json),
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
      timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
      rawEvent: _readRawEvent(json),
    );
  }

  @override
  Map<String, dynamic> toJson() => {
        ...super.toJson(),
        'toolCallId': toolCallId,
        'toolCallName': toolCallName,
        if (parentMessageId != null) 'parentMessageId': parentMessageId,
      };

  // See `_Unset` (top of file) for the sentinel rationale.
  @override
  ToolCallStartEvent copyWith({
    Object? metadata = kUnsetSentinel,
    Object? subagentRunId = kUnsetSentinel,
    String? toolCallId,
    String? toolCallName,
    Object? parentMessageId = kUnsetSentinel,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ToolCallStartEvent(
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      subagentRunId: resolveNullableCopy<String>(
        subagentRunId,
        this.subagentRunId,
      ),
      toolCallId: toolCallId ?? this.toolCallId,
      toolCallName: toolCallName ?? this.toolCallName,
      parentMessageId: identical(parentMessageId, kUnsetSentinel)
          ? this.parentMessageId
          : parentMessageId as String?,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event containing tool call arguments
final class ToolCallArgsEvent extends _SubagentAttributedEvent {
  final String toolCallId;
  final String delta;

  const ToolCallArgsEvent({
    required this.toolCallId,
    required this.delta,
    super.timestamp,
    super.metadata,
    super.subagentRunId,
    super.rawEvent,
  }) : super(eventType: EventType.toolCallArgs);

  factory ToolCallArgsEvent.fromJson(Map<String, dynamic> json) {
    final toolCallId = JsonDecoder.requireEitherField<String>(
      json,
      'toolCallId',
      'tool_call_id',
    );
    // Empty `delta` is accepted to match canonical TS/Python schemas
    // (`ToolCallArgsEventSchema.delta: z.string()` / pydantic `delta: str`).
    final delta = JsonDecoder.requireField<String>(json, 'delta');
    return ToolCallArgsEvent(
      metadata: _readMetadata(json),
      subagentRunId: _readSubagentRunId(json),
      toolCallId: toolCallId,
      delta: delta,
      timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
      rawEvent: _readRawEvent(json),
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
    Object? metadata = kUnsetSentinel,
    Object? subagentRunId = kUnsetSentinel,
    String? toolCallId,
    String? delta,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ToolCallArgsEvent(
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      subagentRunId: resolveNullableCopy<String>(
        subagentRunId,
        this.subagentRunId,
      ),
      toolCallId: toolCallId ?? this.toolCallId,
      delta: delta ?? this.delta,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event indicating the end of a tool call
final class ToolCallEndEvent extends _SubagentAttributedEvent {
  final String toolCallId;

  const ToolCallEndEvent({
    required this.toolCallId,
    super.timestamp,
    super.metadata,
    super.subagentRunId,
    super.rawEvent,
  }) : super(eventType: EventType.toolCallEnd);

  factory ToolCallEndEvent.fromJson(Map<String, dynamic> json) {
    return ToolCallEndEvent(
      metadata: _readMetadata(json),
      subagentRunId: _readSubagentRunId(json),
      toolCallId: JsonDecoder.requireEitherField<String>(
        json,
        'toolCallId',
        'tool_call_id',
      ),
      timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
      rawEvent: _readRawEvent(json),
    );
  }

  @override
  Map<String, dynamic> toJson() => {
        ...super.toJson(),
        'toolCallId': toolCallId,
      };

  @override
  ToolCallEndEvent copyWith({
    Object? metadata = kUnsetSentinel,
    Object? subagentRunId = kUnsetSentinel,
    String? toolCallId,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ToolCallEndEvent(
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      subagentRunId: resolveNullableCopy<String>(
        subagentRunId,
        this.subagentRunId,
      ),
      toolCallId: toolCallId ?? this.toolCallId,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event containing a chunk of tool call content
final class ToolCallChunkEvent extends _SubagentAttributedEvent {
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
    super.metadata,
    super.subagentRunId,
    super.rawEvent,
  }) : super(eventType: EventType.toolCallChunk);

  factory ToolCallChunkEvent.fromJson(Map<String, dynamic> json) {
    return ToolCallChunkEvent(
      metadata: _readMetadata(json),
      subagentRunId: _readSubagentRunId(json),
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
      timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
      rawEvent: _readRawEvent(json),
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

  // See `_Unset` (top of file) for the sentinel rationale.
  @override
  ToolCallChunkEvent copyWith({
    Object? metadata = kUnsetSentinel,
    Object? subagentRunId = kUnsetSentinel,
    Object? toolCallId = kUnsetSentinel,
    Object? toolCallName = kUnsetSentinel,
    Object? parentMessageId = kUnsetSentinel,
    Object? delta = kUnsetSentinel,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ToolCallChunkEvent(
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      subagentRunId: resolveNullableCopy<String>(
        subagentRunId,
        this.subagentRunId,
      ),
      toolCallId: identical(toolCallId, kUnsetSentinel)
          ? this.toolCallId
          : toolCallId as String?,
      toolCallName: identical(toolCallName, kUnsetSentinel)
          ? this.toolCallName
          : toolCallName as String?,
      parentMessageId: identical(parentMessageId, kUnsetSentinel)
          ? this.parentMessageId
          : parentMessageId as String?,
      delta: identical(delta, kUnsetSentinel) ? this.delta : delta as String?,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Role for tool-call result messages (aligned with the AG-UI protocol).
///
/// Currently a single-variant enum mirroring the canonical
/// `Literal["tool"]` (Python) / `z.literal("tool")` (TypeScript). Modeled
/// as an enum so a future role addition can land without churning every
/// call site, and so producers cannot accidentally emit a free-form
/// string like `'developer'` on a `TOOL_CALL_RESULT` event.
enum ToolCallResultRole {
  tool('tool');

  final String value;
  const ToolCallResultRole(this.value);

  /// Parses [value] into a [ToolCallResultRole].
  ///
  /// Throws [ArgumentError] for unknown values. Callers decoding from the
  /// wire should use `ToolCallResultEvent.fromJson`, which absorbs the
  /// throw and falls back to [ToolCallResultRole.tool] so a future
  /// server-side role does not tear down the SSE stream. Mirrors
  /// `ReasoningMessageRole.fromString` and `TextMessageRole.fromString`.
  static final Map<String, ToolCallResultRole> _byValue = Map.unmodifiable({
    for (final r in ToolCallResultRole.values) r.value: r,
  });

  static ToolCallResultRole fromString(String value) {
    return _byValue[value] ??
        (throw ArgumentError('Invalid tool call result role: $value'));
  }
}

/// Event containing the result of a tool call
final class ToolCallResultEvent extends _SubagentAttributedEvent {
  final String messageId;
  final String toolCallId;
  final String content;

  /// Optional role discriminator for the tool-call result.
  ///
  /// `copyWith(role: null)` clears this field via the [kUnsetSentinel]
  /// pattern — same as every other nullable field on this event.
  final ToolCallResultRole? role;

  const ToolCallResultEvent({
    required this.messageId,
    required this.toolCallId,
    required this.content,
    this.role,
    super.timestamp,
    super.metadata,
    super.subagentRunId,
    super.rawEvent,
  }) : super(eventType: EventType.toolCallResult);

  factory ToolCallResultEvent.fromJson(Map<String, dynamic> json) {
    final roleStr = JsonDecoder.optionalField<String>(json, 'role');
    ToolCallResultRole? role;
    if (roleStr != null) {
      try {
        role = ToolCallResultRole.fromString(roleStr);
      } on ArgumentError {
        // Forward-compat: an unknown wire role falls back to `tool` so a
        // future server-side role does not tear down the SSE stream.
        // Mirrors `TextMessageStartEvent.fromJson` /
        // `ReasoningMessageStartEvent.fromJson`. Narrow `on ArgumentError`
        // (not `catch (e)`) preserves propagation of `AGUIValidationError`
        // raised by `optionalField<String>` for a wrong-typed `role`.
        role = ToolCallResultRole.tool;
      }
    }
    return ToolCallResultEvent(
      metadata: _readMetadata(json),
      subagentRunId: _readSubagentRunId(json),
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
      role: role,
      timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
      rawEvent: _readRawEvent(json),
    );
  }

  @override
  Map<String, dynamic> toJson() => {
        ...super.toJson(),
        'messageId': messageId,
        'toolCallId': toolCallId,
        'content': content,
        if (role != null) 'role': role!.value,
      };

  @override
  ToolCallResultEvent copyWith({
    Object? metadata = kUnsetSentinel,
    Object? subagentRunId = kUnsetSentinel,
    String? messageId,
    String? toolCallId,
    String? content,
    Object? role = kUnsetSentinel,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ToolCallResultEvent(
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      subagentRunId: resolveNullableCopy<String>(
        subagentRunId,
        this.subagentRunId,
      ),
      messageId: messageId ?? this.messageId,
      toolCallId: toolCallId ?? this.toolCallId,
      content: content ?? this.content,
      role: identical(role, kUnsetSentinel)
          ? this.role
          : role as ToolCallResultRole?,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

// ============================================================================
// State Events
// ============================================================================

/// Event containing a snapshot of the state
final class StateSnapshotEvent extends _SubagentAttributedEvent {
  /// The state snapshot. Type [State] permits any JSON shape including
  /// `null` (an empty / cleared state is a valid wire payload — see the
  /// matching note on [StateSnapshotEvent.fromJson]).
  final State snapshot;

  const StateSnapshotEvent({
    required this.snapshot,
    super.timestamp,
    super.metadata,
    super.subagentRunId,
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
      metadata: _readMetadata(json),
      subagentRunId: _readSubagentRunId(json),
      snapshot: json['snapshot'],
      timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
      rawEvent: _readRawEvent(json),
    );
  }

  @override
  Map<String, dynamic> toJson() => {
        ...super.toJson(),
        'snapshot': snapshot,
      };

  @override
  StateSnapshotEvent copyWith({
    Object? metadata = kUnsetSentinel,
    Object? subagentRunId = kUnsetSentinel,
    Object? snapshot = kUnsetSentinel,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return StateSnapshotEvent(
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      subagentRunId: resolveNullableCopy<String>(
        subagentRunId,
        this.subagentRunId,
      ),
      snapshot: identical(snapshot, kUnsetSentinel) ? this.snapshot : snapshot,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event containing a delta of the state (JSON Patch RFC 6902)
final class StateDeltaEvent extends _SubagentAttributedEvent {
  // RFC 6902 patch operations are always JSON objects ({op, path, …}).
  // Using List<Map<String, dynamic>> (via requireListField) surfaces
  // non-object elements as AGUIValidationError at the decoder boundary
  // instead of leaking a downstream TypeError at the first op['op'] access.
  final List<Map<String, dynamic>> delta;

  const StateDeltaEvent({
    required this.delta,
    super.timestamp,
    super.metadata,
    super.subagentRunId,
    super.rawEvent,
  }) : super(eventType: EventType.stateDelta);

  factory StateDeltaEvent.fromJson(Map<String, dynamic> json) {
    return StateDeltaEvent(
      metadata: _readMetadata(json),
      subagentRunId: _readSubagentRunId(json),
      delta: JsonDecoder.requireListField<Map<String, dynamic>>(json, 'delta'),
      timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
      rawEvent: _readRawEvent(json),
    );
  }

  @override
  Map<String, dynamic> toJson() => {
        ...super.toJson(),
        'delta': delta,
      };

  @override
  StateDeltaEvent copyWith({
    Object? metadata = kUnsetSentinel,
    Object? subagentRunId = kUnsetSentinel,
    List<Map<String, dynamic>>? delta,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return StateDeltaEvent(
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      subagentRunId: resolveNullableCopy<String>(
        subagentRunId,
        this.subagentRunId,
      ),
      delta: delta ?? this.delta,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event containing a snapshot of messages
///
/// **Sensitive-data warning.** [rawEvent] is automatically cleared (set to
/// `null`) when ANY inner message carries cipher data. This includes every
/// [BaseMessage] subtype ([ReasoningMessage], [AssistantMessage],
/// [ToolMessage], [SystemMessage], [DeveloperMessage], [UserMessage]) with a
/// non-null [encryptedValue], AND any `role: 'activity'` entry whose wire form
/// carries an `encryptedValue` / `encrypted_value` key (which
/// [ActivityMessage.fromJson] silently strips from the structured field).
/// This prevents the verbatim wire map — which may include cipher data — from
/// leaking through [BaseEvent.rawEvent] to log sinks or reflection-based
/// serializers. Proxy operators that need the verbatim wire form should keep
/// their own copy of the raw JSON before calling [fromJson].
/// See [ReasoningEncryptedValueEvent.fromJson] for the same pattern on
/// individual cipher events.
final class MessagesSnapshotEvent extends BaseEvent {
  final List<Message> messages;

  MessagesSnapshotEvent({
    required this.messages,
    super.timestamp,
    super.metadata,
    super.rawEvent,
  }) : super(eventType: EventType.messagesSnapshot) {
    // Direct-construction caveat: this guard only inspects the structured
    // Message.encryptedValue field. A caller that already has a wire-form
    // rawEvent map whose payload contains an encryptedValue key on an
    // activity-role entry can still violate the cipher-scrub invariant —
    // ActivityMessage.encryptedValue is always null by construction, so it
    // cannot be detected here. Pass rawEvent: null or pre-scrub the map before
    // invoking this constructor for activity-role cipher data. fromJson enforces
    // both code paths; this constructor enforces only the structured-field one.
    if (rawEvent != null &&
        containsEncryptedValue(messages.map((m) => m.toJson()).toList())) {
      throw AGUIValidationError(
        message: 'Direct construction with rawEvent + cipher-bearing messages '
            'violates the scrub invariant. Pass rawEvent: null or pre-scrub.',
        field: 'rawEvent',
      );
    }
  }

  factory MessagesSnapshotEvent.fromJson(Map<String, dynamic> json) {
    final rawMessages = JsonDecoder.requireListField<Map<String, dynamic>>(
      json,
      'messages',
    );
    final messages = <Message>[];
    for (var i = 0; i < rawMessages.length; i++) {
      try {
        messages.add(Message.fromJson(rawMessages[i]));
      } catch (e) {
        if (e is AGUIValidationError) {
          // Always drop json: — the inner Message map can carry encryptedValue
          // for Tool/Reasoning subtypes. Preserve cause: only when the inner
          // error already cleared its own json: field (e.json == null), which
          // indicates the inner factory was cipher-aware and the cause chain
          // does not expose raw wire data. Non-cipher messages (Developer,
          // System, User) typically produce errors with e.json == null, so
          // their cause is preserved for ergonomic debugging.
          throw AGUIValidationError(
            message: e.message,
            field: e.field != null ? 'messages[$i].${e.field}' : 'messages[$i]',
            value: e.value,
            cause: e.json == null ? e : null,
          );
        }
        throw AGUIValidationError(
          message: 'Failed to decode message at index $i: $e',
          field: 'messages[$i]',
          cause: e,
        );
      }
    }
    // Auto-scrub rawEvent when any inner message carries cipher data. Storing
    // the verbatim wire map in rawEvent would undo the cipher scrubbing that
    // the ReasoningMessage factory already applied to the structured field.
    // Proxies that need the verbatim wire form should keep their own copy of
    // the raw JSON before calling fromJson.
    //
    // ActivityMessage.fromJson silently strips wire-level encryptedValue from
    // the structured field (the constructor does not accept it), so the
    // structured-field predicate alone would miss a cipher on an
    // ActivityMessage. We check rawMessages directly for role == 'activity'
    // entries that still carry a cipher key on the wire.
    //
    // SCRUB CONTRACT: this check assumes encryptedValue / encrypted_value is
    // the only cipher-named key on any Message subtype. If a future subtype
    // adds a different sensitive payload key, this hasCipher predicate MUST be
    // extended in parallel.
    final hasCipher = containsEncryptedValue(rawMessages);
    return MessagesSnapshotEvent(
      metadata: _readMetadata(json),
      messages: messages,
      timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
      rawEvent: hasCipher ? null : _readRawEvent(json),
    );
  }

  @override
  Map<String, dynamic> toJson() => {
        ...super.toJson(),
        'messages': messages.map((m) => m.toJson()).toList(),
      };

  /// Creates a copy of this event with the given fields replaced.
  ///
  /// **Cipher-safety note.** When the resolved messages list does NOT carry
  /// cipher data (`encryptedValue == null` for every message), [rawEvent] is
  /// applied normally — kept, cleared, or replaced per the standard sentinel
  /// semantics. When ANY resolved message carries cipher data
  /// (`encryptedValue != null`), [rawEvent] is silently forced to `null`
  /// regardless of the supplied value. This mirrors the `fromJson` invariant
  /// that prevents the raw wire map (which contains the cipher payload) from
  /// leaking through `rawEvent`. Callers that have already scrubbed
  /// `encryptedValue` and need to retain a sanitized `rawEvent` map should
  /// construct a new [MessagesSnapshotEvent] directly with
  /// `rawEvent: scrubbedMap` rather than using `copyWith`.
  @override
  MessagesSnapshotEvent copyWith({
    Object? metadata = kUnsetSentinel,
    List<Message>? messages,
    int? timestamp,
    Object? rawEvent = kUnsetSentinel,
  }) {
    final newMessages = messages ?? this.messages;
    // Re-apply the fromJson cipher-scrub invariant: if any message in the
    // (possibly updated) list carries cipher data, force rawEvent to null so
    // the wire map cannot be reattached and expose encrypted content.
    // ActivityMessage always returns null for encryptedValue by construction;
    // see SCRUB CONTRACT comment in fromJson.
    final hasCipher =
        containsEncryptedValue(newMessages.map((m) => m.toJson()).toList());
    // Log in all builds (including release) when a caller passes a non-null
    // rawEvent that will be silently scrubbed. The force-to-null below is
    // the authoritative safety measure; the log helps callers diagnose
    // unexpected scrub in production without crashing.
    if (hasCipher && !identical(rawEvent, kUnsetSentinel) && rawEvent != null) {
      developer.log(
        'MessagesSnapshotEvent.copyWith: rawEvent is silently forced to null '
        'when any message carries encryptedValue. Construct directly if you '
        'need to retain a sanitized rawEvent.',
        name: 'ag_ui.cipher_scrub',
        level: 900, // WARNING
      );
    }
    final dynamic resolvedRaw;
    if (hasCipher) {
      resolvedRaw = null;
    } else if (identical(rawEvent, kUnsetSentinel)) {
      resolvedRaw = this.rawEvent;
    } else {
      resolvedRaw = rawEvent;
    }
    return MessagesSnapshotEvent(
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      messages: newMessages,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: resolvedRaw,
    );
  }
}
