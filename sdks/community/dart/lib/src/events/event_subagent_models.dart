part of 'events.dart';

SubagentFinishedOutcome? _readSubagentFinishedOutcome(
  Map<String, dynamic> json,
) {
  if (!json.containsKey('outcome') || json['outcome'] == null) {
    return null;
  }
  try {
    return SubagentFinishedOutcome.fromJson(
      JsonDecoder.requireField<Map<String, dynamic>>(json, 'outcome'),
    );
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

/// Announces a subagent invocation within the current root run.
final class SubagentStartedEvent extends BaseEvent {
  const SubagentStartedEvent({
    required this.subagentRunId,
    required this.name,
    this.description,
    this.parentSubagentRunId,
    this.parentToolCallId,
    this.parentMessageId,
    super.timestamp,
    super.metadata,
    super.rawEvent,
  }) : super(eventType: EventType.subagentStarted);

  factory SubagentStartedEvent.fromJson(Map<String, dynamic> json) {
    try {
      return SubagentStartedEvent(
        subagentRunId: JsonDecoder.requireEitherField<String>(
          json,
          'subagentRunId',
          'subagent_run_id',
        ),
        name: JsonDecoder.requireField<String>(json, 'name'),
        description: JsonDecoder.optionalField<String>(json, 'description'),
        parentSubagentRunId: JsonDecoder.optionalEitherField<String>(
          json,
          'parentSubagentRunId',
          'parent_subagent_run_id',
        ),
        parentToolCallId: JsonDecoder.optionalEitherField<String>(
          json,
          'parentToolCallId',
          'parent_tool_call_id',
        ),
        parentMessageId: JsonDecoder.optionalEitherField<String>(
          json,
          'parentMessageId',
          'parent_message_id',
        ),
        timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
        metadata: _readMetadata(json),
        rawEvent: _readRawEvent(json),
      );
    } on AGUIValidationError catch (error) {
      throw sanitizeValidationError(enclosingJson: json, error: error);
    }
  }

  final String subagentRunId;
  final String name;
  final String? description;
  final String? parentSubagentRunId;
  final String? parentToolCallId;
  final String? parentMessageId;

  @override
  Map<String, dynamic> toJson() => {
        ...super.toJson(),
        'subagentRunId': subagentRunId,
        'name': name,
        if (description != null) 'description': description,
        if (parentSubagentRunId != null)
          'parentSubagentRunId': parentSubagentRunId,
        if (parentToolCallId != null) 'parentToolCallId': parentToolCallId,
        if (parentMessageId != null) 'parentMessageId': parentMessageId,
      };

  @override
  SubagentStartedEvent copyWith({
    String? subagentRunId,
    String? name,
    Object? description = kUnsetSentinel,
    Object? parentSubagentRunId = kUnsetSentinel,
    Object? parentToolCallId = kUnsetSentinel,
    Object? parentMessageId = kUnsetSentinel,
    int? timestamp,
    Object? metadata = kUnsetSentinel,
    dynamic rawEvent,
  }) {
    return SubagentStartedEvent(
      subagentRunId: subagentRunId ?? this.subagentRunId,
      name: name ?? this.name,
      description: resolveNullableCopy<String>(description, this.description),
      parentSubagentRunId: resolveNullableCopy<String>(
        parentSubagentRunId,
        this.parentSubagentRunId,
      ),
      parentToolCallId: resolveNullableCopy<String>(
        parentToolCallId,
        this.parentToolCallId,
      ),
      parentMessageId: resolveNullableCopy<String>(
        parentMessageId,
        this.parentMessageId,
      ),
      timestamp: timestamp ?? this.timestamp,
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Closes one subagent stream segment without terminating the root run.
final class SubagentFinishedEvent extends BaseEvent {
  const SubagentFinishedEvent({
    required this.subagentRunId,
    this.result,
    this.outcome,
    super.timestamp,
    super.metadata,
    super.rawEvent,
  }) : super(eventType: EventType.subagentFinished);

  factory SubagentFinishedEvent.fromJson(Map<String, dynamic> json) {
    try {
      return SubagentFinishedEvent(
        subagentRunId: JsonDecoder.requireEitherField<String>(
          json,
          'subagentRunId',
          'subagent_run_id',
        ),
        result: json['result'],
        outcome: _readSubagentFinishedOutcome(json),
        timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
        metadata: _readMetadata(json),
        rawEvent: _readRawEvent(json),
      );
    } on AGUIValidationError catch (error) {
      throw sanitizeValidationError(enclosingJson: json, error: error);
    }
  }

  final String subagentRunId;

  /// Optional arbitrary JSON completion value. Null normalizes to omission.
  final dynamic result;
  final SubagentFinishedOutcome? outcome;

  @override
  Map<String, dynamic> toJson() => {
        ...super.toJson(),
        'subagentRunId': subagentRunId,
        if (result != null) 'result': result,
        if (outcome != null) 'outcome': outcome!.toJson(),
      };

  @override
  SubagentFinishedEvent copyWith({
    String? subagentRunId,
    Object? result = kUnsetSentinel,
    Object? outcome = kUnsetSentinel,
    int? timestamp,
    Object? metadata = kUnsetSentinel,
    dynamic rawEvent,
  }) {
    return SubagentFinishedEvent(
      subagentRunId: subagentRunId ?? this.subagentRunId,
      result: identical(result, kUnsetSentinel) ? this.result : result,
      outcome: resolveNullableCopy<SubagentFinishedOutcome>(
        outcome,
        this.outcome,
      ),
      timestamp: timestamp ?? this.timestamp,
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}

/// Reports a subagent failure without converting it to a root run error.
final class SubagentErrorEvent extends BaseEvent {
  const SubagentErrorEvent({
    required this.subagentRunId,
    required this.message,
    this.code,
    super.timestamp,
    super.metadata,
    super.rawEvent,
  }) : super(eventType: EventType.subagentError);

  factory SubagentErrorEvent.fromJson(Map<String, dynamic> json) {
    try {
      return SubagentErrorEvent(
        subagentRunId: JsonDecoder.requireEitherField<String>(
          json,
          'subagentRunId',
          'subagent_run_id',
        ),
        message: JsonDecoder.requireField<String>(json, 'message'),
        code: JsonDecoder.optionalField<String>(json, 'code'),
        timestamp: JsonDecoder.optionalIntField(json, 'timestamp'),
        metadata: _readMetadata(json),
        rawEvent: _readRawEvent(json),
      );
    } on AGUIValidationError catch (error) {
      throw sanitizeValidationError(enclosingJson: json, error: error);
    }
  }

  final String subagentRunId;
  final String message;
  final String? code;

  @override
  Map<String, dynamic> toJson() => {
        ...super.toJson(),
        'subagentRunId': subagentRunId,
        'message': message,
        if (code != null) 'code': code,
      };

  @override
  SubagentErrorEvent copyWith({
    String? subagentRunId,
    String? message,
    Object? code = kUnsetSentinel,
    int? timestamp,
    Object? metadata = kUnsetSentinel,
    dynamic rawEvent,
  }) {
    return SubagentErrorEvent(
      subagentRunId: subagentRunId ?? this.subagentRunId,
      message: message ?? this.message,
      code: resolveNullableCopy<String>(code, this.code),
      timestamp: timestamp ?? this.timestamp,
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      rawEvent: rawEvent ?? this.rawEvent,
    );
  }
}
