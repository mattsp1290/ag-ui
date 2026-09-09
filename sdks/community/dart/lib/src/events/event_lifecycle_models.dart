part of 'events.dart';

// ============================================================================
// Lifecycle Events
// ============================================================================

/// Event indicating that a run has started
final class RunStartedEvent extends BaseEvent {
  final String threadId;
  final String runId;
  final String? parentRunId;

  /// Optional `RUN_STARTED` input snapshot. On the wire the `input` key
  /// must hold a JSON object — `optionalField<Map<String, dynamic>>` in
  /// [RunStartedEvent.fromJson] rejects a wrong-typed value (string, list,
  /// number, etc.) with `AGUIValidationError(field: 'input')`. An absent
  /// or explicit-null `input` decodes as `null`.
  final RunAgentInput? input;

  RunStartedEvent({
    required this.threadId,
    required this.runId,
    this.parentRunId,
    this.input,
    super.timestamp,
    super.metadata,
    super.rawEvent,
  }) : super(eventType: EventType.runStarted) {
    if (rawEvent != null &&
        input != null &&
        containsEncryptedValue(input!.toJson())) {
      throw AGUIValidationError(
        message:
            'Direct construction with rawEvent + cipher-bearing input.messages '
            'violates the scrub invariant. Pass rawEvent: null or pre-scrub.',
        field: 'rawEvent',
      );
    }
  }

  factory RunStartedEvent.fromJson(Map<String, dynamic> json) {
    final inputJson = JsonDecoder.optionalField<Map<String, dynamic>>(
      json,
      'input',
    );
    RunAgentInput? input;
    if (inputJson != null) {
      try {
        input = RunAgentInput.fromJson(inputJson);
      } on AGUIValidationError catch (e) {
        // Omit json: — e.json (the inner RunAgentInput payload) can carry
        // encryptedValue via input.messages[*]. Omit cause: for the same
        // reason: the cause chain exposes e.json to reflection-based log
        // shippers. Surface only the field path and the non-cipher value.
        throw AGUIValidationError(
          message: e.message,
          field: e.field != null ? 'input.${e.field}' : 'input',
          value: e.value,
        );
      }
    }
    // Auto-scrub rawEvent when any input message carries cipher data, mirroring
    // the MessagesSnapshotEvent.fromJson invariant.
    //
    // Inspect the raw input recursively so ActivityMessage wire fields and
    // nested ToolCall cipher values are covered before their containing models
    // normalize the payload.
    //
    // Scope note: this predicate only sweeps input.messages (the structured
    // RunAgentInput). If a malformed payload omits `input` entirely but carries
    // encrypted material under a top-level key, that material is not caught
    // here. The attack surface is narrow (requires a malformed payload AND an
    // absent `input` key) and asymmetric with MessagesSnapshotEvent by design:
    // RunStartedEvent only encrypts the input.messages path.
    //
    // SCRUB CONTRACT: encryptedValue / encrypted_value are the protocol's
    // cipher-bearing keys at every nesting depth.
    final hasCipher = inputJson != null && containsEncryptedValue(inputJson);
    return RunStartedEvent(
      metadata: _readMetadata(json),
      threadId: JsonDecoder.requireEitherField<String>(
        json,
        'threadId',
        'thread_id',
      ),
      runId: JsonDecoder.requireEitherField<String>(json, 'runId', 'run_id'),
      parentRunId: JsonDecoder.optionalEitherField<String>(
        json,
        'parentRunId',
        'parent_run_id',
      ),
      input: input,
      timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
      rawEvent: hasCipher ? null : _readRawEvent(json),
    );
  }

  @override
  Map<String, dynamic> toJson() => {
        ...super.toJson(),
        'threadId': threadId,
        'runId': runId,
        if (parentRunId != null) 'parentRunId': parentRunId,
        if (input != null) 'input': input!.toJson(),
      };

  /// Creates a copy of this event with the given fields replaced.
  ///
  /// **Cipher-safety note.** If any message in the resolved `input.messages`
  /// list carries cipher data (`encryptedValue != null`), the `rawEvent`
  /// parameter is silently forced to `null` regardless of the value the caller
  /// supplies. This mirrors the `fromJson` invariant that prevents the raw wire
  /// map from leaking cipher payloads through `rawEvent`. Callers that have
  /// already scrubbed a sanitized `rawEvent` map should construct a new
  /// [RunStartedEvent] directly with `rawEvent: scrubbedMap` rather than
  /// calling `copyWith`.
  // See `_Unset` (top of file) for the sentinel rationale.
  @override
  RunStartedEvent copyWith({
    Object? metadata = kUnsetSentinel,
    String? threadId,
    String? runId,
    Object? parentRunId = kUnsetSentinel,
    Object? input = kUnsetSentinel,
    int? timestamp,
    Object? rawEvent = kUnsetSentinel,
  }) {
    // The sentinel check MUST come before the type check — if input IS the
    // sentinel, `input is! RunAgentInput?` evaluates true (Object is not
    // RunAgentInput?) and would incorrectly throw. Swapping the two conditions
    // breaks all no-arg copyWith() calls.
    if (!identical(input, kUnsetSentinel) && input is! RunAgentInput?) {
      throw ArgumentError.value(
        input,
        'input',
        'must be RunAgentInput?, null, or kUnsetSentinel',
      );
    }
    final newInput =
        identical(input, kUnsetSentinel) ? this.input : input as RunAgentInput?;
    // Re-apply the fromJson cipher-scrub invariant on the resolved input.
    final hasCipher =
        newInput != null && containsEncryptedValue(newInput.toJson());
    // Log in all builds (including release) when a caller passes a non-null
    // rawEvent that will be silently scrubbed. The force-to-null below is
    // the authoritative safety measure; the log helps callers diagnose
    // unexpected scrub in production without crashing.
    if (hasCipher && !identical(rawEvent, kUnsetSentinel) && rawEvent != null) {
      developer.log(
        'RunStartedEvent.copyWith: rawEvent is silently forced to null '
        'when any input message carries encryptedValue. Construct directly if '
        'you need to retain a sanitized rawEvent.',
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
    return RunStartedEvent(
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      threadId: threadId ?? this.threadId,
      runId: runId ?? this.runId,
      parentRunId: identical(parentRunId, kUnsetSentinel)
          ? this.parentRunId
          : parentRunId as String?,
      input: newInput,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: resolvedRaw,
    );
  }
}

/// Event indicating that a run has finished
final class RunFinishedEvent extends BaseEvent {
  final String threadId;
  final String runId;
  final RunFinishedOutcome? outcome;

  /// Optional run-completion payload (`z.any().optional()` /
  /// `Optional[Any] = None` in TS/Python). On the wire, an explicit
  /// `'result': null` and an absent `result` key are equivalent — both
  /// produce a [RunFinishedEvent] with `result == null`, and [toJson]
  /// drops the key when `result` is null.
  ///
  /// The [kUnsetSentinel] on [copyWith] (`Object? result = kUnsetSentinel`)
  /// is for in-memory disambiguation only — it lets callers explicitly clear
  /// a previously-set result without constructing a new event. It is NOT a
  /// wire-protocol distinction: both `null` and absent produce identical
  /// `toJson` output (key omitted). Do not mirror the
  /// `ActivitySnapshotEvent.content` always-emit pattern here; the protocol
  /// does not require [RunFinishedEvent.result] on the wire. If you need the
  /// distinction visible in the wire output, construct a new [RunFinishedEvent]
  /// directly with the field always emitted.
  final dynamic result;

  const RunFinishedEvent({
    required this.threadId,
    required this.runId,
    this.result,
    this.outcome,
    super.timestamp,
    super.metadata,
    super.rawEvent,
  }) : super(eventType: EventType.runFinished);

  factory RunFinishedEvent.fromJson(Map<String, dynamic> json) {
    // Unlike StateSnapshotEvent / RawEvent / CustomEvent / ActivitySnapshotEvent
    // which use containsKey to enforce key presence, `result` is truly optional
    // (canonical `z.any().optional()` / `Optional[Any] = None`). An absent key
    // and an explicit `'result': null` are equivalent — both produce `result == null`.
    try {
      return RunFinishedEvent(
        metadata: _readMetadata(json),
        threadId: JsonDecoder.requireEitherField<String>(
          json,
          'threadId',
          'thread_id',
        ),
        runId: JsonDecoder.requireEitherField<String>(json, 'runId', 'run_id'),
        result: json['result'],
        outcome: _readRunFinishedOutcome(json),
        timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
        rawEvent: _readRawEvent(json),
      );
    } on AGUIValidationError catch (error) {
      throw sanitizeValidationError(
        enclosingJson: json,
        error: error,
      );
    }
  }

  @override
  Map<String, dynamic> toJson() => {
        ...super.toJson(),
        'threadId': threadId,
        'runId': runId,
        if (result != null) 'result': result,
        if (outcome != null) 'outcome': outcome!.toJson(),
      };

  // See `_Unset` (top of file) for the sentinel rationale.
  @override
  RunFinishedEvent copyWith({
    Object? metadata = kUnsetSentinel,
    String? threadId,
    String? runId,
    Object? result = kUnsetSentinel,
    Object? outcome = kUnsetSentinel,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return RunFinishedEvent(
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      threadId: threadId ?? this.threadId,
      runId: runId ?? this.runId,
      result: identical(result, kUnsetSentinel) ? this.result : result,
      outcome: identical(outcome, kUnsetSentinel)
          ? this.outcome
          : outcome as RunFinishedOutcome?,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event indicating that a run has encountered an error
final class RunErrorEvent extends BaseEvent {
  final String message;

  /// Optional machine-readable error code.
  final String? code;

  const RunErrorEvent({
    required this.message,
    this.code,
    super.timestamp,
    super.metadata,
    super.rawEvent,
  }) : super(eventType: EventType.runError);

  factory RunErrorEvent.fromJson(Map<String, dynamic> json) {
    return RunErrorEvent(
      metadata: _readMetadata(json),
      message: JsonDecoder.requireField<String>(json, 'message'),
      code: JsonDecoder.optionalField<String>(json, 'code'),
      timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
      rawEvent: _readRawEvent(json),
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
    Object? metadata = kUnsetSentinel,
    String? message,
    Object? code = kUnsetSentinel,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return RunErrorEvent(
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      message: message ?? this.message,
      code: identical(code, kUnsetSentinel) ? this.code : code as String?,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event indicating that a step has started
final class StepStartedEvent extends _SubagentAttributedEvent {
  final String stepName;

  const StepStartedEvent({
    required this.stepName,
    super.timestamp,
    super.metadata,
    super.subagentRunId,
    super.rawEvent,
  }) : super(eventType: EventType.stepStarted);

  factory StepStartedEvent.fromJson(Map<String, dynamic> json) {
    return StepStartedEvent(
      metadata: _readMetadata(json),
      subagentRunId: _readSubagentRunId(json),
      stepName: JsonDecoder.requireEitherField<String>(
        json,
        'stepName',
        'step_name',
      ),
      timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
      rawEvent: _readRawEvent(json),
    );
  }

  @override
  Map<String, dynamic> toJson() => {
        ...super.toJson(),
        'stepName': stepName,
      };

  @override
  StepStartedEvent copyWith({
    Object? metadata = kUnsetSentinel,
    Object? subagentRunId = kUnsetSentinel,
    String? stepName,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return StepStartedEvent(
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      subagentRunId: resolveNullableCopy<String>(
        subagentRunId,
        this.subagentRunId,
      ),
      stepName: stepName ?? this.stepName,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Event indicating that a step has finished
final class StepFinishedEvent extends _SubagentAttributedEvent {
  final String stepName;

  const StepFinishedEvent({
    required this.stepName,
    super.timestamp,
    super.metadata,
    super.subagentRunId,
    super.rawEvent,
  }) : super(eventType: EventType.stepFinished);

  factory StepFinishedEvent.fromJson(Map<String, dynamic> json) {
    return StepFinishedEvent(
      metadata: _readMetadata(json),
      subagentRunId: _readSubagentRunId(json),
      stepName: JsonDecoder.requireEitherField<String>(
        json,
        'stepName',
        'step_name',
      ),
      timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
      rawEvent: _readRawEvent(json),
    );
  }

  @override
  Map<String, dynamic> toJson() => {
        ...super.toJson(),
        'stepName': stepName,
      };

  @override
  StepFinishedEvent copyWith({
    Object? metadata = kUnsetSentinel,
    Object? subagentRunId = kUnsetSentinel,
    String? stepName,
    int? timestamp,
    dynamic rawEvent,
  }) {
    return StepFinishedEvent(
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      subagentRunId: resolveNullableCopy<String>(
        subagentRunId,
        this.subagentRunId,
      ),
      stepName: stepName ?? this.stepName,
      timestamp: timestamp ?? this.timestamp,
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}
