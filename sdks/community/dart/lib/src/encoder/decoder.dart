/// Event decoder for AG-UI protocol.
///
/// Decodes wire format (SSE or binary) to Dart models.
library;

import 'dart:convert';
import 'dart:typed_data';

import '../client/errors.dart';
import '../client/validators.dart';
import '../events/events.dart';
import '../types/base.dart';

/// Decoder for AG-UI events.
///
/// Supports decoding events from SSE (Server-Sent Events) format
/// and binary format (protobuf or SSE as bytes).
class EventDecoder {
  /// Creates a decoder instance.
  const EventDecoder();

  /// Decodes an event from a string (assumed to be JSON).
  ///
  /// This method expects a JSON string without the SSE "data: " prefix.
  BaseEvent decode(String data) {
    try {
      final decoded = jsonDecode(data);
      // Validate the top-level shape explicitly so a list/primitive
      // payload (`[1,2,3]`, `"hello"`, `42`) produces a structured
      // [DecodingError] instead of a `TypeError` swallowed by the
      // catch-all below — which was being wrapped as a generic "Failed
      // to decode event" with no hint about the actual mismatch.
      if (decoded is! Map<String, dynamic>) {
        throw DecodingError(
          'Expected JSON object at top level',
          field: 'data',
          expectedType: 'Map<String, dynamic>',
          actualValue: decoded,
        );
      }
      return decodeJson(decoded);
    } on FormatException catch (e) {
      throw DecodingError(
        'Invalid JSON format',
        field: 'data',
        expectedType: 'JSON',
        actualValue: data,
        cause: e,
      );
    } on AgUiError {
      rethrow;
    } catch (e) {
      throw DecodingError(
        'Failed to decode event',
        field: 'event',
        expectedType: 'BaseEvent',
        actualValue: data,
        cause: e,
      );
    }
  }

  /// Decodes an event from a JSON map.
  BaseEvent decodeJson(Map<String, dynamic> json) {
    try {
      // `BaseEvent.fromJson` already enforces presence and string-type
      // for the `type` discriminator via `JsonDecoder.requireField<String>`,
      // and `validate()` below enforces non-empty on identifier strings.
      // No standalone pre-check needed — keeping one collapsed the
      // `type: 123` (wrong-typed) path into a single `AGUIValidationError`
      // wrapped uniformly into [DecodingError] by the handlers below.
      final event = BaseEvent.fromJson(json);

      // Validate the created event
      validate(event);

      return event;
    } on ValidationError catch (e) {
      // Wire-boundary contract documented on `AGUIValidationError`
      // (lib/src/types/base.dart): both `AGUIValidationError` (from
      // `fromJson` factories) and `ValidationError` (from `validate()`
      // via `Validators.requireNonEmpty`) surface to consumers as
      // `DecodingError` so callers only need to catch one error type at
      // the decode boundary. This `on` clause covers the
      // `AgUiError`-extending sibling so it does not bypass the wrapping
      // via the `on AgUiError` rethrow.
      throw _wrapValidation(e, e.field, json);
    } on AGUIValidationError catch (e) {
      // Companion clause for the factory-side error. Without this branch,
      // `AGUIValidationError` (which only `implements Exception`, not
      // `AgUiError`) falls through to the catch-all below and the
      // original failing field — `role`, `messageId`, `subtype`, etc. —
      // is flattened to `field: 'json'`, breaking the public decoder
      // error surface.
      throw _wrapValidation(e, e.field, json);
    } on AgUiError {
      rethrow;
    } catch (e) {
      throw DecodingError(
        'Failed to create event from JSON',
        field: 'json',
        expectedType: 'BaseEvent',
        actualValue: json,
        cause: e,
      );
    }
  }

  /// Decodes an SSE message.
  ///
  /// Expects a complete SSE message with "data: " prefix and double newlines.
  BaseEvent decodeSSE(String sseMessage) {
    // Extract data from SSE format
    final lines = sseMessage.split('\n');
    final dataLines = <String>[];
    
    for (final line in lines) {
      if (line.startsWith('data: ')) {
        dataLines.add(line.substring(6)); // Remove "data: " prefix
      } else if (line.startsWith('data:')) {
        dataLines.add(line.substring(5)); // Remove "data:" prefix
      }
    }
    
    if (dataLines.isEmpty) {
      throw DecodingError(
        'No data found in SSE message',
        field: 'sseMessage',
        expectedType: 'SSE with data field',
        actualValue: sseMessage,
      );
    }
    
    // Join all data lines (for multi-line data)
    final data = dataLines.join('\n');
    
    // Handle special SSE comment for keep-alive
    if (data.trim() == ':') {
      throw DecodingError(
        'SSE keep-alive comment, not an event',
        field: 'data',
        expectedType: 'JSON event data',
        actualValue: data,
      );
    }
    
    return decode(data);
  }

  /// Decodes an event from binary data.
  ///
  /// Currently assumes the binary data is UTF-8 encoded SSE.
  /// TODO: Add protobuf support when proto definitions are available.
  BaseEvent decodeBinary(Uint8List data) {
    try {
      final string = utf8.decode(data);
      
      // Check if it looks like SSE format
      if (string.startsWith('data:')) {
        return decodeSSE(string);
      } else {
        // Assume it's raw JSON
        return decode(string);
      }
    } on FormatException catch (e) {
      throw DecodingError(
        'Invalid UTF-8 data',
        field: 'binary',
        expectedType: 'UTF-8 encoded data',
        actualValue: data,
        cause: e,
      );
    }
  }

  /// Validates that an event has all required fields.
  ///
  /// Defensive re-check on top of `fromJson`: catches empty-string values
  /// (which `JsonDecoder.requireField<String>` permits), and any event
  /// constructed outside `fromJson` (e.g. via a `copyWith` that violates
  /// the non-empty contract). The asymmetry is intentional — `fromJson`
  /// only enforces presence and type; `validate()` is the single source of
  /// truth for non-empty constraints on string identifiers.
  ///
  /// Returns true if valid, throws [ValidationError] if not.
  bool validate(BaseEvent event) {
    // Basic validation - ensure type is set
    Validators.validateEventType(event.type);
    
    // Type-specific validation. Listing every sealed subtype explicitly
    // (no `default`) makes the analyzer flag any new event type that is
    // added without a corresponding decision here. When you add a case
    // here, also update `BaseEvent.fromJson` in
    // `lib/src/events/events.dart` so the discriminator-dispatch switch
    // and this validator remain in sync.
    switch (event) {
      case TextMessageStartEvent():
        Validators.requireNonEmpty(event.messageId, 'messageId');
      case TextMessageContentEvent():
        Validators.requireNonEmpty(event.messageId, 'messageId');
        Validators.requireNonEmpty(event.delta, 'delta');
      case TextMessageEndEvent():
        Validators.requireNonEmpty(event.messageId, 'messageId');
      case TextMessageChunkEvent():
        break;
      case ThinkingTextMessageStartEvent():
        break;
      case ThinkingTextMessageContentEvent():
        break;
      case ThinkingTextMessageEndEvent():
        break;
      case ToolCallStartEvent():
        Validators.requireNonEmpty(event.toolCallId, 'toolCallId');
        Validators.requireNonEmpty(event.toolCallName, 'toolCallName');
      case ToolCallArgsEvent():
        Validators.requireNonEmpty(event.toolCallId, 'toolCallId');
        Validators.requireNonEmpty(event.delta, 'delta');
      case ToolCallEndEvent():
        Validators.requireNonEmpty(event.toolCallId, 'toolCallId');
      case ToolCallChunkEvent():
        break;
      case ToolCallResultEvent():
        Validators.requireNonEmpty(event.messageId, 'messageId');
        Validators.requireNonEmpty(event.toolCallId, 'toolCallId');
        Validators.requireNonEmpty(event.content, 'content');
      case ThinkingStartEvent():
        break;
      // ignore: deprecated_member_use_from_same_package
      case ThinkingContentEvent():
        Validators.requireNonEmpty(event.delta, 'delta');
      case ThinkingEndEvent():
        break;
      case StateSnapshotEvent():
        // `snapshot` is an opaque JSON value — presence is enforced in
        // `StateSnapshotEvent.fromJson`; there is no non-empty constraint
        // we can express on `dynamic` content here.
        break;
      case StateDeltaEvent():
        break;
      case MessagesSnapshotEvent():
        break;
      case ActivitySnapshotEvent():
        Validators.requireNonEmpty(event.messageId, 'messageId');
        Validators.requireNonEmpty(event.activityType, 'activityType');
      case ActivityDeltaEvent():
        Validators.requireNonEmpty(event.messageId, 'messageId');
        Validators.requireNonEmpty(event.activityType, 'activityType');
      case RawEvent():
        // `event` payload presence is enforced in `RawEvent.fromJson`.
        break;
      case CustomEvent():
        Validators.requireNonEmpty(event.name, 'name');
      case RunStartedEvent():
        Validators.validateThreadId(event.threadId);
        Validators.validateRunId(event.runId);
      case RunFinishedEvent():
        Validators.validateThreadId(event.threadId);
        Validators.validateRunId(event.runId);
      case RunErrorEvent():
        Validators.requireNonEmpty(event.message, 'message');
      case StepStartedEvent():
        Validators.requireNonEmpty(event.stepName, 'stepName');
      case StepFinishedEvent():
        Validators.requireNonEmpty(event.stepName, 'stepName');
      case ReasoningStartEvent():
        Validators.requireNonEmpty(event.messageId, 'messageId');
      case ReasoningMessageStartEvent():
        Validators.requireNonEmpty(event.messageId, 'messageId');
      case ReasoningMessageContentEvent():
        Validators.requireNonEmpty(event.messageId, 'messageId');
        Validators.requireNonEmpty(event.delta, 'delta');
      case ReasoningMessageEndEvent():
        Validators.requireNonEmpty(event.messageId, 'messageId');
      case ReasoningMessageChunkEvent():
        break;
      case ReasoningEndEvent():
        Validators.requireNonEmpty(event.messageId, 'messageId');
      case ReasoningEncryptedValueEvent():
        // `subtype` is enum-typed and constructor-required, so it cannot
        // be null/invalid here. If the enum ever gains an `unknown`
        // member (currently `fromString` throws — see the dartdoc on
        // `ReasoningEncryptedValueSubtype.fromString`), this case is the
        // place to reject it.
        Validators.requireNonEmpty(event.entityId, 'entityId');
        Validators.requireNonEmpty(event.encryptedValue, 'encryptedValue');
    }

    return true;
  }

  /// Wraps a factory-side or validate-side validation failure into the
  /// public [DecodingError] envelope, preserving the original failing
  /// field name so consumers can react to specific field violations
  /// instead of getting a flattened `field: 'json'` everywhere.
  DecodingError _wrapValidation(
    Object cause,
    String? field,
    Map<String, dynamic> json,
  ) {
    return DecodingError(
      'Failed to create event from JSON',
      field: field ?? 'json',
      expectedType: 'BaseEvent',
      actualValue: json,
      cause: cause,
    );
  }
}