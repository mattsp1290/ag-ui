part of 'events.dart';

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
  static final Map<String, ReasoningMessageRole> _byValue = Map.unmodifiable({
    for (final r in ReasoningMessageRole.values) r.value: r,
  });

  static ReasoningMessageRole fromString(String value) {
    return _byValue[value] ??
        (throw ArgumentError('Invalid reasoning message role: $value'));
  }
}

/// Subtype for [ReasoningEncryptedValueEvent].
enum ReasoningEncryptedValueSubtype {
  /// Wire spelling is `'tool-call'` with a hyphen — canonical across the
  /// AG-UI protocol (Python `Literal["tool-call"]`, TypeScript
  /// `z.literal("tool-call")`). The Dart symbol is `toolCall`; the dash is
  /// intentional, not a typo.
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
  static final Map<String, ReasoningEncryptedValueSubtype> _byValue =
      Map.unmodifiable({
    for (final s in ReasoningEncryptedValueSubtype.values) s.value: s,
  });

  /// Throws [AGUIValidationError] (not [ArgumentError]) on unknown values —
  /// unlike [EventType.fromString] which throws [ArgumentError] so that
  /// `BaseEvent.fromJson`'s narrow `on ArgumentError` catch can distinguish
  /// unknown event types from factory bugs. Subtype is a cipher-data
  /// discriminator with no safe fallback; throwing [AGUIValidationError]
  /// directly surfaces it uniformly as [DecodingError] through the decoder
  /// pipeline without a wrapping layer.
  static ReasoningEncryptedValueSubtype fromString(String value) {
    return _byValue[value] ??
        (throw AGUIValidationError(
          message: 'Invalid reasoning encrypted value subtype: $value',
          field: 'subtype',
          value: value,
          // Intentionally omit json: — this helper is called from cipher-data path.
        ));
  }
}

/// Event indicating the start of a reasoning phase.
final class ReasoningStartEvent extends _SubagentAttributedEvent {
  final String messageId;

  const ReasoningStartEvent({
    required this.messageId,
    super.timestamp,
    super.metadata,
    super.subagentRunId,
    super.rawEvent,
  }) : super(eventType: EventType.reasoningStart);

  factory ReasoningStartEvent.fromJson(Map<String, dynamic> json) {
    return ReasoningStartEvent(
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
  ReasoningStartEvent copyWith({
    Object? metadata = kUnsetSentinel,
    Object? subagentRunId = kUnsetSentinel,
    String? messageId,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ReasoningStartEvent(
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

/// Event indicating the start of a reasoning message.
final class ReasoningMessageStartEvent extends _SubagentAttributedEvent {
  final String messageId;
  final ReasoningMessageRole role;

  const ReasoningMessageStartEvent({
    required this.messageId,
    this.role = ReasoningMessageRole.reasoning,
    super.timestamp,
    super.metadata,
    super.subagentRunId,
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
      metadata: _readMetadata(json),
      subagentRunId: _readSubagentRunId(json),
      messageId: messageId,
      role: role,
      timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
      rawEvent: _readRawEvent(json),
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
    Object? metadata = kUnsetSentinel,
    Object? subagentRunId = kUnsetSentinel,
    String? messageId,
    ReasoningMessageRole? role,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ReasoningMessageStartEvent(
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      subagentRunId: resolveNullableCopy<String>(
        subagentRunId,
        this.subagentRunId,
      ),
      messageId: messageId ?? this.messageId,
      role: role ?? this.role,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event containing a piece of reasoning message content.
final class ReasoningMessageContentEvent extends _SubagentAttributedEvent {
  final String messageId;
  final String delta;

  const ReasoningMessageContentEvent({
    required this.messageId,
    required this.delta,
    super.timestamp,
    super.metadata,
    super.subagentRunId,
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
    // Empty `delta` is accepted to match canonical TS/Python schemas
    // (`ReasoningMessageContentEventSchema.delta: z.string()` /
    // pydantic `delta: str`).
    final delta = JsonDecoder.requireField<String>(json, 'delta');

    return ReasoningMessageContentEvent(
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
  ReasoningMessageContentEvent copyWith({
    Object? metadata = kUnsetSentinel,
    Object? subagentRunId = kUnsetSentinel,
    String? messageId,
    String? delta,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ReasoningMessageContentEvent(
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

/// Event indicating the end of a reasoning message.
final class ReasoningMessageEndEvent extends _SubagentAttributedEvent {
  final String messageId;

  const ReasoningMessageEndEvent({
    required this.messageId,
    super.timestamp,
    super.metadata,
    super.subagentRunId,
    super.rawEvent,
  }) : super(eventType: EventType.reasoningMessageEnd);

  factory ReasoningMessageEndEvent.fromJson(Map<String, dynamic> json) {
    return ReasoningMessageEndEvent(
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
  ReasoningMessageEndEvent copyWith({
    Object? metadata = kUnsetSentinel,
    Object? subagentRunId = kUnsetSentinel,
    String? messageId,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ReasoningMessageEndEvent(
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

/// Event containing a chunk of reasoning message content.
final class ReasoningMessageChunkEvent extends _SubagentAttributedEvent {
  final String? messageId;
  final String? delta;

  const ReasoningMessageChunkEvent({
    this.messageId,
    this.delta,
    super.timestamp,
    super.metadata,
    super.subagentRunId,
    super.rawEvent,
  }) : super(eventType: EventType.reasoningMessageChunk);

  factory ReasoningMessageChunkEvent.fromJson(Map<String, dynamic> json) {
    return ReasoningMessageChunkEvent(
      metadata: _readMetadata(json),
      subagentRunId: _readSubagentRunId(json),
      messageId: JsonDecoder.optionalEitherField<String>(
        json,
        'messageId',
        'message_id',
      ),
      delta: JsonDecoder.optionalField<String>(json, 'delta'),
      timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
      rawEvent: _readRawEvent(json),
    );
  }

  @override
  Map<String, dynamic> toJson() => {
        ...super.toJson(),
        if (messageId != null) 'messageId': messageId,
        if (delta != null) 'delta': delta,
      };

  // See `_Unset` (top of file) for the sentinel rationale.
  @override
  ReasoningMessageChunkEvent copyWith({
    Object? metadata = kUnsetSentinel,
    Object? subagentRunId = kUnsetSentinel,
    Object? messageId = kUnsetSentinel,
    Object? delta = kUnsetSentinel,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ReasoningMessageChunkEvent(
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      subagentRunId: resolveNullableCopy<String>(
        subagentRunId,
        this.subagentRunId,
      ),
      messageId: identical(messageId, kUnsetSentinel)
          ? this.messageId
          : messageId as String?,
      delta: identical(delta, kUnsetSentinel) ? this.delta : delta as String?,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event indicating the end of a reasoning phase.
final class ReasoningEndEvent extends _SubagentAttributedEvent {
  final String messageId;

  const ReasoningEndEvent({
    required this.messageId,
    super.timestamp,
    super.metadata,
    super.subagentRunId,
    super.rawEvent,
  }) : super(eventType: EventType.reasoningEnd);

  factory ReasoningEndEvent.fromJson(Map<String, dynamic> json) {
    return ReasoningEndEvent(
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
  ReasoningEndEvent copyWith({
    Object? metadata = kUnsetSentinel,
    Object? subagentRunId = kUnsetSentinel,
    String? messageId,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return ReasoningEndEvent(
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

// ---------------------------------------------------------------------------
// Cipher-safe field extraction helper
// ---------------------------------------------------------------------------

/// Extracts a required [String] field without including [json] in any thrown
/// [AGUIValidationError] — use only for cipher-data payloads where forwarding
/// the raw map through log-shippers or reflection-based serializers would leak
/// sensitive data.
///
/// [camelKey] is tried first (camelCase-wins precedence matching
/// [JsonDecoder.requireEitherField]). [snakeKey], if given, is the fallback.
String _requireCipherSafeString(
  Map<String, dynamic> json,
  String camelKey, [
  String? snakeKey,
]) {
  final bool present = json.containsKey(camelKey) ||
      (snakeKey != null && json.containsKey(snakeKey));
  final rawValue = json.containsKey(camelKey) ? json[camelKey] : json[snakeKey];

  if (!present) {
    throw AGUIValidationError(
      message: snakeKey != null
          ? 'Missing required field "$camelKey" (or "$snakeKey")'
          : 'Missing required field "$camelKey"',
      field: camelKey,
      // Intentionally omit json: — payload contains cipher data.
    );
  }
  if (rawValue == null) {
    throw AGUIValidationError(
      message: 'Field "$camelKey" must not be null',
      field: camelKey,
      // Intentionally omit json: — payload contains cipher data.
    );
  }
  if (rawValue is! String) {
    throw AGUIValidationError(
      message:
          'Field "$camelKey" has incorrect type. Expected String, got ${rawValue.runtimeType}',
      field: camelKey,
      // Record only the runtime type, not the raw value — payload contains
      // cipher data; even a wrong-typed value could be sensitive material.
      value: rawValue.runtimeType.toString(),
      // Intentionally omit json: — payload contains cipher data.
    );
  }
  return rawValue;
}

/// Event containing an encrypted value for a message or tool call.
///
/// **Cipher-safety guarantees.** All three wire fields ([subtype], [entityId],
/// [encryptedValue]) are parsed via the internal `_requireCipherSafeString`
/// helper, which omits `json:` from every thrown [AGUIValidationError] so
/// cipher payloads cannot leak through reflection-based error serializers or
/// log shippers. [BaseEvent.rawEvent] is unconditionally set to `null` in
/// `fromJson` — the verbatim wire map carries `encryptedValue` and must not
/// be re-exposed downstream. The `copyWith` method intentionally omits a
/// `rawEvent` parameter for the same reason; if you need a scrubbed
/// `rawEvent`, construct a new [ReasoningEncryptedValueEvent] directly.
///
/// **Forward-compat note.** A future server-side [subtype] value will cause
/// [ReasoningEncryptedValueSubtype.fromString] to throw, which propagates
/// out of `fromJson` as an [AGUIValidationError] (wrapped in a
/// [DecodingError] when reached through [EventDecoder]). To keep streams
/// alive across an unknown subtype, opt in to per-event recovery via
/// `EventStreamAdapter(skipInvalidEvents: true)` — the rest of the SDK's
/// enums absorb unknown values at the event-decoding boundary, but the
/// encrypted-payload subtype has no sensible default to fall back to.
final class ReasoningEncryptedValueEvent extends _SubagentAttributedEvent {
  final ReasoningEncryptedValueSubtype subtype;
  final String entityId;
  final String encryptedValue;

  // SECURITY: `rawEvent` is intentionally absent from this constructor.
  // Pinning it to null in the super-initializer prevents callers from
  // re-attaching a wire map (which carries `encryptedValue`) and bypassing
  // the cipher-scrub invariant enforced in `fromJson` and `copyWith`.
  const ReasoningEncryptedValueEvent({
    required this.subtype,
    required this.entityId,
    required this.encryptedValue,
    super.timestamp,
    super.metadata,
    super.subagentRunId,
  }) : super(eventType: EventType.reasoningEncryptedValue, rawEvent: null);

  factory ReasoningEncryptedValueEvent.fromJson(Map<String, dynamic> json) {
    // All three required fields use [_requireCipherSafeString] rather than
    // `requireField`/`requireEitherField` so that every error path omits
    // `json:` — the payload contains cipher data and forwarding the full wire
    // map to `AGUIValidationError.json` would leak it through reflection-based
    // error serializers and log shippers. See [_requireCipherSafeString].
    final subtypeRaw = _requireCipherSafeString(json, 'subtype');
    // fromString now throws AGUIValidationError directly on unknown values,
    // so no try/catch wrapper is needed — the error propagates correctly
    // through the decoder pipeline to DecodingError.
    final subtype = ReasoningEncryptedValueSubtype.fromString(subtypeRaw);

    // entityId and encryptedValue are accepted as plain strings (including
    // empty) to match canonical schemas: TS `z.string()` and Python `str`
    // (no `min_length`). The strict subtype discriminator above stays —
    // unknown subtypes still throw.
    final entityId = _requireCipherSafeString(json, 'entityId', 'entity_id');
    final encryptedValue = _requireCipherSafeString(
      json,
      'encryptedValue',
      'encrypted_value',
    );

    // rawEvent is explicitly set to null — unlike every other factory in this
    // file, forwarding _readRawEvent(json) would store the full cipher payload
    // in BaseEvent.rawEvent, undoing all the cipher-data scrubbing above.
    // Proxies that need the raw wire form should maintain their own copy before
    // calling fromJson.
    return ReasoningEncryptedValueEvent(
      metadata: _readMetadata(json),
      subagentRunId: _readSubagentRunId(json),
      subtype: subtype,
      entityId: entityId,
      encryptedValue: encryptedValue,
      timestamp: JsonDecoder.optionalCipherSafeIntField(json, 'timestamp'),
      // rawEvent: omitted — constructor pins rawEvent: null in its super-initializer.
    );
  }

  @override
  Map<String, dynamic> toJson() => {
        ...super.toJson(),
        'subtype': subtype.value,
        'entityId': entityId,
        'encryptedValue': encryptedValue,
      };

  // SECURITY: `rawEvent` is intentionally omitted from `copyWith`.
  // `fromJson` always pins `rawEvent: null` to prevent the cipher payload
  // in `encryptedValue` from leaking through the raw wire map. Accepting
  // a `rawEvent` parameter here would let callers re-attach that map and
  // undo the scrub — `MessagesSnapshotEvent.copyWith` applies the same
  // restriction for the same reason. Callers that genuinely need a non-null
  // `rawEvent` (e.g. a proxy that has already stripped `encryptedValue`)
  // must construct a new `ReasoningEncryptedValueEvent` directly.
  @override
  ReasoningEncryptedValueEvent copyWith({
    Object? metadata = kUnsetSentinel,
    Object? subagentRunId = kUnsetSentinel,
    ReasoningEncryptedValueSubtype? subtype,
    String? entityId,
    String? encryptedValue,
    int? timestamp,
  }) {
    // The three `?? this.field` reads are safe — unlike nullable fields that use
    // the kUnsetSentinel discipline elsewhere, these are required non-nullable
    // constructor parameters, so `this.subtype`, `this.entityId`, and
    // `this.encryptedValue` are always non-null. Passing null for any of them
    // silently preserves the existing value; it cannot clear a required field.
    return ReasoningEncryptedValueEvent(
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      subagentRunId: resolveNullableCopy<String>(
        subagentRunId,
        this.subagentRunId,
      ),
      subtype: subtype ?? this.subtype,
      entityId: entityId ?? this.entityId,
      encryptedValue: encryptedValue ?? this.encryptedValue,
      timestamp: timestamp ?? this.timestamp,
      // rawEvent: always null — enforced by the constructor's super-initializer.
    );
  }
}
