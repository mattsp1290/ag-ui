/// Tool-related types for AG-UI protocol.
///
/// This library defines types for tool interactions, including tool calls
/// from the assistant and tool definitions.
library;

import 'base.dart';

// Sentinel used by copyWith to distinguish "argument omitted" from
// "argument explicitly null" on nullable fields. Mirrors the same
// pattern in lib/src/types/message.dart and lib/src/events/events.dart.
class _Unset {
  const _Unset();
}

const _Unset _unsetTool = _Unset();

/// Represents a function call within a tool call.
///
/// Contains the function name and serialized arguments for execution.
class FunctionCall extends AGUIModel {
  final String name;
  final String arguments;

  const FunctionCall({
    required this.name,
    required this.arguments,
  });

  factory FunctionCall.fromJson(Map<String, dynamic> json) {
    return FunctionCall(
      name: JsonDecoder.requireField<String>(json, 'name'),
      arguments: JsonDecoder.requireField<String>(json, 'arguments'),
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    'name': name,
    'arguments': arguments,
  };

  @override
  FunctionCall copyWith({
    String? name,
    String? arguments,
  }) {
    return FunctionCall(
      name: name ?? this.name,
      arguments: arguments ?? this.arguments,
    );
  }
}

/// Represents a tool call made by the assistant.
///
/// Tool calls allow the assistant to request execution of external functions
/// or tools to gather information or perform actions.
///
/// The optional [encryptedValue] is an opaque cipher payload that a Dart
/// proxy must forward verbatim. It mirrors the canonical TS/Python
/// `ToolCall.encryptedValue` / `ToolCall.encrypted_value` field.
class ToolCall extends AGUIModel {
  final String id;
  final String type;
  final FunctionCall function;
  final String? encryptedValue;

  const ToolCall({
    required this.id,
    this.type = 'function',
    required this.function,
    this.encryptedValue,
  });

  factory ToolCall.fromJson(Map<String, dynamic> json) {
    return ToolCall(
      id: JsonDecoder.requireField<String>(json, 'id'),
      type: JsonDecoder.optionalField<String>(json, 'type') ?? 'function',
      function: FunctionCall.fromJson(
        JsonDecoder.requireField<Map<String, dynamic>>(json, 'function'),
      ),
      encryptedValue: JsonDecoder.optionalEitherField<String>(
        json,
        'encryptedValue',
        'encrypted_value',
      ),
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    'id': id,
    'type': type,
    'function': function.toJson(),
    if (encryptedValue != null) 'encryptedValue': encryptedValue,
  };

  // `encryptedValue` is nullable — sentinel lets callers clear it
  // explicitly. Mirrors the message-class sentinel in
  // lib/src/types/message.dart.
  @override
  ToolCall copyWith({
    String? id,
    String? type,
    FunctionCall? function,
    Object? encryptedValue = _unsetTool,
  }) {
    return ToolCall(
      id: id ?? this.id,
      type: type ?? this.type,
      function: function ?? this.function,
      encryptedValue: identical(encryptedValue, _unsetTool)
          ? this.encryptedValue
          : encryptedValue as String?,
    );
  }
}

/// Represents a tool definition.
///
/// Defines a tool that can be called by the assistant, including its
/// name, description, and parameter schema.
class Tool extends AGUIModel {
  final String name;
  final String description;
  final dynamic parameters; // JSON Schema for the tool parameters

  const Tool({
    required this.name,
    required this.description,
    this.parameters,
  });

  factory Tool.fromJson(Map<String, dynamic> json) {
    return Tool(
      name: JsonDecoder.requireField<String>(json, 'name'),
      description: JsonDecoder.requireField<String>(json, 'description'),
      parameters: json['parameters'], // Allow any JSON Schema
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    'name': name,
    'description': description,
    if (parameters != null) 'parameters': parameters,
  };

  // `parameters` is nullable (any JSON Schema shape) — sentinel lets
  // callers clear it explicitly via `copyWith(parameters: null)`. Mirrors
  // the sentinel discipline on `ToolCall.encryptedValue` in the same file.
  @override
  Tool copyWith({
    String? name,
    String? description,
    Object? parameters = _unsetTool,
  }) {
    return Tool(
      name: name ?? this.name,
      description: description ?? this.description,
      parameters: identical(parameters, _unsetTool) ? this.parameters : parameters,
    );
  }
}

/// Represents the result of a tool call
class ToolResult extends AGUIModel {
  final String toolCallId;
  final String content;
  final String? error;

  const ToolResult({
    required this.toolCallId,
    required this.content,
    this.error,
  });

  factory ToolResult.fromJson(Map<String, dynamic> json) {
    return ToolResult(
      toolCallId: JsonDecoder.requireEitherField<String>(
        json,
        'toolCallId',
        'tool_call_id',
      ),
      content: JsonDecoder.requireField<String>(json, 'content'),
      error: JsonDecoder.optionalField<String>(json, 'error'),
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    'toolCallId': toolCallId,
    'content': content,
    if (error != null) 'error': error,
  };

  // `error` is nullable — sentinel lets callers clear it explicitly via
  // `copyWith(error: null)`. Mirrors `ToolCall.encryptedValue` above.
  @override
  ToolResult copyWith({
    String? toolCallId,
    String? content,
    Object? error = _unsetTool,
  }) {
    return ToolResult(
      toolCallId: toolCallId ?? this.toolCallId,
      content: content ?? this.content,
      error: identical(error, _unsetTool) ? this.error : error as String?,
    );
  }
}