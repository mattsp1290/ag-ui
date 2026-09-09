/// Interrupt and resume models for the AG-UI protocol.
library;

import 'base.dart';
import 'copy_utils.dart';
import 'metadata.dart';
import 'wire_safety.dart';

Metadata? _readMetadata(Map<String, dynamic> json) =>
    readCipherAwareOptionalField<Map<String, dynamic>>(json, 'metadata');

/// A pause point that a caller may resolve in a later run request.
class Interrupt extends AGUIModel {
  const Interrupt({
    required this.id,
    required this.reason,
    this.message,
    this.toolCallId,
    this.responseSchema,
    this.expiresAt,
    this.metadata,
    this.subagentRunId,
  });

  factory Interrupt.fromJson(Map<String, dynamic> json) {
    try {
      return Interrupt(
        id: JsonDecoder.requireField<String>(json, 'id'),
        reason: JsonDecoder.requireField<String>(json, 'reason'),
        message: JsonDecoder.optionalField<String>(json, 'message'),
        toolCallId: JsonDecoder.optionalEitherField<String>(
          json,
          'toolCallId',
          'tool_call_id',
        ),
        responseSchema: JsonDecoder.optionalEitherField<Map<String, dynamic>>(
          json,
          'responseSchema',
          'response_schema',
        ),
        expiresAt: JsonDecoder.optionalEitherField<String>(
          json,
          'expiresAt',
          'expires_at',
        ),
        metadata: _readMetadata(json),
        subagentRunId: readCipherAwareOptionalEitherField<String>(
          json,
          'subagentRunId',
          'subagent_run_id',
        ),
      );
    } on AGUIValidationError catch (error) {
      throw sanitizeValidationError(
        enclosingJson: json,
        error: error,
      );
    }
  }

  final String id;
  final String reason;
  final String? message;
  final String? toolCallId;
  final Map<String, dynamic>? responseSchema;
  final String? expiresAt;
  final Metadata? metadata;
  final String? subagentRunId;

  @override
  Map<String, dynamic> toJson() => {
        'id': id,
        'reason': reason,
        if (message != null) 'message': message,
        if (toolCallId != null) 'toolCallId': toolCallId,
        if (responseSchema != null) 'responseSchema': responseSchema,
        if (expiresAt != null) 'expiresAt': expiresAt,
        if (metadata != null) 'metadata': metadata,
        if (subagentRunId != null) 'subagentRunId': subagentRunId,
      };

  @override
  Interrupt copyWith({
    String? id,
    String? reason,
    Object? message = kUnsetSentinel,
    Object? toolCallId = kUnsetSentinel,
    Object? responseSchema = kUnsetSentinel,
    Object? expiresAt = kUnsetSentinel,
    Object? metadata = kUnsetSentinel,
    Object? subagentRunId = kUnsetSentinel,
  }) {
    return Interrupt(
      id: id ?? this.id,
      reason: reason ?? this.reason,
      message: resolveNullableCopy<String>(message, this.message),
      toolCallId: resolveNullableCopy<String>(toolCallId, this.toolCallId),
      responseSchema: resolveNullableCopy<Map<String, dynamic>>(
        responseSchema,
        this.responseSchema,
      ),
      expiresAt: resolveNullableCopy<String>(expiresAt, this.expiresAt),
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
      subagentRunId: resolveNullableCopy<String>(
        subagentRunId,
        this.subagentRunId,
      ),
    );
  }
}

/// Resolution state for an [Interrupt].
enum ResumeStatus {
  resolved('resolved'),
  cancelled('cancelled');

  const ResumeStatus(this.value);

  final String value;

  static ResumeStatus fromString(String value) {
    for (final status in values) {
      if (status.value == value) {
        return status;
      }
    }
    throw AGUIValidationError(
      message: 'Unknown resume status: $value',
      field: 'status',
      value: value,
    );
  }
}

/// A caller response to one interrupt in a resumed run request.
class ResumeEntry extends AGUIModel {
  const ResumeEntry({
    required this.interruptId,
    required this.status,
    this.payload,
    this.metadata,
  });

  factory ResumeEntry.fromJson(Map<String, dynamic> json) {
    try {
      return ResumeEntry(
        interruptId: JsonDecoder.requireEitherField<String>(
          json,
          'interruptId',
          'interrupt_id',
        ),
        status: ResumeStatus.fromString(
          JsonDecoder.requireField<String>(json, 'status'),
        ),
        payload: json['payload'],
        metadata: _readMetadata(json),
      );
    } on AGUIValidationError catch (error) {
      throw sanitizeValidationError(
        enclosingJson: json,
        error: error,
      );
    }
  }

  final String interruptId;
  final ResumeStatus status;
  final dynamic payload;
  final Metadata? metadata;

  @override
  Map<String, dynamic> toJson() => {
        'interruptId': interruptId,
        'status': status.value,
        if (payload != null) 'payload': payload,
        if (metadata != null) 'metadata': metadata,
      };

  @override
  ResumeEntry copyWith({
    String? interruptId,
    ResumeStatus? status,
    Object? payload = kUnsetSentinel,
    Object? metadata = kUnsetSentinel,
  }) {
    return ResumeEntry(
      interruptId: interruptId ?? this.interruptId,
      status: status ?? this.status,
      payload: resolveNullableCopy<dynamic>(payload, this.payload),
      metadata: resolveNullableCopy<Metadata>(metadata, this.metadata),
    );
  }
}
