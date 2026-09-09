part of 'events.dart';

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
final class ActivitySnapshotEvent extends _SubagentAttributedEvent {
  final String messageId;
  final String activityType;
  final Object? content;

  /// `true` (the default) means this snapshot replaces any prior content
  /// for the same [messageId]; `false` means it merges/extends.
  ///
  /// Optional on the wire (`replace: z.boolean().optional().default(true)`
  /// in TS, `replace: bool = True` in Python). [toJson] omits the field
  /// when it equals the default `true`, matching canonical TypeScript and
  /// Python wire output. `fromJson` restores the default when the field is
  /// absent, so round-trip semantics are preserved.
  final bool replace;

  const ActivitySnapshotEvent({
    required this.messageId,
    required this.activityType,
    required this.content,
    this.replace = true,
    super.timestamp,
    super.metadata,
    super.subagentRunId,
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
      metadata: _readMetadata(json),
      subagentRunId: _readSubagentRunId(json),
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
      timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
      rawEvent: _readRawEvent(json),
    );
  }

  @override
  Map<String, dynamic> toJson() => {
        ...super.toJson(),
        'messageId': messageId,
        'activityType': activityType,
        'content': content,
        // Omit `replace` when it equals the default `true`, matching canonical
        // TS/Python wire output. `fromJson` defaults to `true` when absent, so
        // round-trip semantics are preserved.
        if (!replace) 'replace': replace,
      };

  // See `_Unset` (top of file) for the sentinel rationale.
  @override
  ActivitySnapshotEvent copyWith({
    Object? metadata = kUnsetSentinel,
    Object? subagentRunId = kUnsetSentinel,
    String? messageId,
    String? activityType,
    Object? content = kUnsetSentinel,
    bool? replace,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ActivitySnapshotEvent(
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      subagentRunId: resolveNullableCopy<String>(
        subagentRunId,
        this.subagentRunId,
      ),
      messageId: messageId ?? this.messageId,
      activityType: activityType ?? this.activityType,
      content: identical(content, kUnsetSentinel) ? this.content : content,
      replace: replace ?? this.replace,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event containing a JSON Patch (RFC 6902) delta for an activity message
final class ActivityDeltaEvent extends _SubagentAttributedEvent {
  final String messageId;
  final String activityType;
  // RFC 6902 patch operations are always JSON objects ({op, path, …}).
  // Using List<Map<String, dynamic>> (via requireListField) surfaces
  // non-object elements as AGUIValidationError at the decoder boundary.
  final List<Map<String, dynamic>> patch;

  const ActivityDeltaEvent({
    required this.messageId,
    required this.activityType,
    required this.patch,
    super.timestamp,
    super.metadata,
    super.subagentRunId,
    super.rawEvent,
  }) : super(eventType: EventType.activityDelta);

  factory ActivityDeltaEvent.fromJson(Map<String, dynamic> json) {
    return ActivityDeltaEvent(
      metadata: _readMetadata(json),
      subagentRunId: _readSubagentRunId(json),
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
      patch: JsonDecoder.requireListField<Map<String, dynamic>>(json, 'patch'),
      timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
      rawEvent: _readRawEvent(json),
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
    Object? metadata = kUnsetSentinel,
    Object? subagentRunId = kUnsetSentinel,
    String? messageId,
    String? activityType,
    List<Map<String, dynamic>>? patch,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ActivityDeltaEvent(
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      subagentRunId: resolveNullableCopy<String>(
        subagentRunId,
        this.subagentRunId,
      ),
      messageId: messageId ?? this.messageId,
      activityType: activityType ?? this.activityType,
      patch: patch ?? this.patch,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event wrapping a raw, uninterpreted upstream event payload.
///
/// Three related but distinct concepts coexist on this class:
/// - [eventType]: always `EventType.raw` — the discriminator that routes wire
///   payloads here via `BaseEvent.fromJson`.
/// - [event]: the raw upstream event payload as decoded from the wire JSON
///   `event` field. May be any JSON shape, including `null`.
/// - [rawEvent]: inherited from [BaseEvent] — the verbatim wire JSON of the
///   *enclosing* SSE message (the whole `{type, event, ...}` map). Populated
///   by `_readRawEvent` when the producer includes a `rawEvent` /
///   `raw_event` key. Unrelated to the [event] field above.
final class RawEvent extends _SubagentAttributedEvent {
  final dynamic event;
  final String? source;

  const RawEvent({
    required this.event,
    this.source,
    super.timestamp,
    super.metadata,
    super.subagentRunId,
    super.rawEvent,
  }) : super(eventType: EventType.raw);

  /// Decodes a [RawEvent] from a JSON map.
  ///
  /// **Cross-SDK note.** The `event` key MUST be present on the wire — this
  /// Dart SDK aligns with the Python `event: Any` (required) schema rather
  /// than the TypeScript `z.any()` schema which permits `undefined` (i.e.
  /// an absent key). A TypeScript server that omits the `event` key entirely
  /// will be rejected with `AGUIValidationError(field: 'event')`.
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
      metadata: _readMetadata(json),
      subagentRunId: _readSubagentRunId(json),
      event: json['event'],
      source: JsonDecoder.optionalField<String>(json, 'source'),
      timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
      rawEvent: _readRawEvent(json),
    );
  }

  @override
  Map<String, dynamic> toJson() => {
        ...super.toJson(),
        'event': event,
        if (source != null) 'source': source,
      };

  // See `_Unset` (top of file) for the sentinel rationale. Both `event`
  // and `source` are nullable on the wire, so callers need explicit-clear
  // semantics to drop a stale upstream payload.
  @override
  RawEvent copyWith({
    Object? metadata = kUnsetSentinel,
    Object? subagentRunId = kUnsetSentinel,
    Object? event = kUnsetSentinel,
    Object? source = kUnsetSentinel,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return RawEvent(
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      subagentRunId: resolveNullableCopy<String>(
        subagentRunId,
        this.subagentRunId,
      ),
      event: identical(event, kUnsetSentinel) ? this.event : event,
      source:
          identical(source, kUnsetSentinel) ? this.source : source as String?,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event containing a custom event
final class CustomEvent extends _SubagentAttributedEvent {
  final String name;
  final dynamic value;

  const CustomEvent({
    required this.name,
    required this.value,
    super.timestamp,
    super.metadata,
    super.subagentRunId,
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
      metadata: _readMetadata(json),
      subagentRunId: _readSubagentRunId(json),
      name: JsonDecoder.requireField<String>(json, 'name'),
      value: json['value'],
      timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
      rawEvent: _readRawEvent(json),
    );
  }

  @override
  Map<String, dynamic> toJson() => {
        ...super.toJson(),
        'name': name,
        'value': value,
      };

  // See `_Unset` (top of file) for the sentinel rationale.
  @override
  CustomEvent copyWith({
    Object? metadata = kUnsetSentinel,
    Object? subagentRunId = kUnsetSentinel,
    String? name,
    Object? value = kUnsetSentinel,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return CustomEvent(
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      subagentRunId: resolveNullableCopy<String>(
        subagentRunId,
        this.subagentRunId,
      ),
      name: name ?? this.name,
      value: identical(value, kUnsetSentinel) ? this.value : value,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}
