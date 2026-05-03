/// Message types for AG-UI protocol.
///
/// This library defines the message types used in agent-user conversations,
/// including user, assistant, system, tool, developer, activity, and
/// reasoning messages.
library;

import 'base.dart';
import 'tool.dart';

/// Role types for messages in the AG-UI protocol.
///
/// Mirrors the canonical TypeScript and Python `Message` discriminated
/// unions (see `sdks/typescript/packages/core/src/types.ts` and
/// `sdks/python/ag_ui/core/types.py`). The `activity` and `reasoning`
/// values exist so `MESSAGES_SNAPSHOT` payloads carrying those message
/// shapes decode in Dart with the same schema as the other SDKs.
enum MessageRole {
  developer('developer'),
  system('system'),
  assistant('assistant'),
  user('user'),
  tool('tool'),
  activity('activity'),
  reasoning('reasoning');

  final String value;
  const MessageRole(this.value);

  static MessageRole fromString(String value) {
    return MessageRole.values.firstWhere(
      (role) => role.value == value,
      orElse: () => throw AGUIValidationError(
        message: 'Invalid message role: $value',
        field: 'role',
        value: value,
      ),
    );
  }
}

/// Base message class for all message types.
///
/// Messages represent the fundamental units of conversation in the AG-UI protocol.
/// Each message has a role, optional content, and may include additional metadata.
///
/// Use the [Message.fromJson] factory to deserialize messages from JSON.
sealed class Message extends AGUIModel with TypeDiscriminator {
  final String? id;
  final MessageRole role;
  final String? content;
  final String? name;

  const Message({
    this.id,
    required this.role,
    this.content,
    this.name,
  });

  @override
  String get type => role.value;

  /// Factory constructor to create specific message types from JSON
  factory Message.fromJson(Map<String, dynamic> json) {
    final roleStr = JsonDecoder.requireField<String>(json, 'role');
    final role = MessageRole.fromString(roleStr);

    switch (role) {
      case MessageRole.developer:
        return DeveloperMessage.fromJson(json);
      case MessageRole.system:
        return SystemMessage.fromJson(json);
      case MessageRole.assistant:
        return AssistantMessage.fromJson(json);
      case MessageRole.user:
        return UserMessage.fromJson(json);
      case MessageRole.tool:
        return ToolMessage.fromJson(json);
      case MessageRole.activity:
        return ActivityMessage.fromJson(json);
      case MessageRole.reasoning:
        return ReasoningMessage.fromJson(json);
    }
  }

  @override
  Map<String, dynamic> toJson() => {
    if (id != null) 'id': id,
    'role': role.value,
    if (content != null) 'content': content,
    if (name != null) 'name': name,
  };
}

/// Developer message with required content.
///
/// Used for system-level or developer-facing messages in the conversation.
class DeveloperMessage extends Message {
  @override
  final String content;

  const DeveloperMessage({
    required super.id,
    required this.content,
    super.name,
  }) : super(role: MessageRole.developer);

  factory DeveloperMessage.fromJson(Map<String, dynamic> json) {
    return DeveloperMessage(
      id: JsonDecoder.requireField<String>(json, 'id'),
      content: JsonDecoder.requireField<String>(json, 'content'),
      name: JsonDecoder.optionalField<String>(json, 'name'),
    );
  }

  @override
  DeveloperMessage copyWith({
    String? id,
    String? content,
    String? name,
  }) {
    return DeveloperMessage(
      id: id ?? this.id,
      content: content ?? this.content,
      name: name ?? this.name,
    );
  }
}

/// System message with required content.
///
/// Represents system-level instructions or context provided to the agent.
class SystemMessage extends Message {
  @override
  final String content;

  const SystemMessage({
    required super.id,
    required this.content,
    super.name,
  }) : super(role: MessageRole.system);

  factory SystemMessage.fromJson(Map<String, dynamic> json) {
    return SystemMessage(
      id: JsonDecoder.requireField<String>(json, 'id'),
      content: JsonDecoder.requireField<String>(json, 'content'),
      name: JsonDecoder.optionalField<String>(json, 'name'),
    );
  }

  @override
  SystemMessage copyWith({
    String? id,
    String? content,
    String? name,
  }) {
    return SystemMessage(
      id: id ?? this.id,
      content: content ?? this.content,
      name: name ?? this.name,
    );
  }
}

/// Assistant message with optional content and tool calls.
///
/// Represents responses from the AI assistant, which may include
/// text content and/or tool call requests.
class AssistantMessage extends Message {
  final List<ToolCall>? toolCalls;

  const AssistantMessage({
    required super.id,
    super.content,
    super.name,
    this.toolCalls,
  }) : super(role: MessageRole.assistant);

  factory AssistantMessage.fromJson(Map<String, dynamic> json) {
    return AssistantMessage(
      id: JsonDecoder.requireField<String>(json, 'id'),
      content: JsonDecoder.optionalField<String>(json, 'content'),
      name: JsonDecoder.optionalField<String>(json, 'name'),
      toolCalls: JsonDecoder.optionalListField<Map<String, dynamic>>(
        json,
        'toolCalls',
      )?.map((item) => ToolCall.fromJson(item)).toList() ??
        JsonDecoder.optionalListField<Map<String, dynamic>>(
          json,
          'tool_calls',
        )?.map((item) => ToolCall.fromJson(item)).toList(),
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    if (toolCalls != null && toolCalls!.isNotEmpty) 
      'toolCalls': toolCalls!.map((tc) => tc.toJson()).toList(),
  };

  @override
  AssistantMessage copyWith({
    String? id,
    String? content,
    String? name,
    List<ToolCall>? toolCalls,
  }) {
    return AssistantMessage(
      id: id ?? this.id,
      content: content ?? this.content,
      name: name ?? this.name,
      toolCalls: toolCalls ?? this.toolCalls,
    );
  }
}

/// User message with required content.
///
/// Represents input from the user in the conversation.
class UserMessage extends Message {
  @override
  final String content;

  const UserMessage({
    required super.id,
    required this.content,
    super.name,
  }) : super(role: MessageRole.user);

  factory UserMessage.fromJson(Map<String, dynamic> json) {
    return UserMessage(
      id: JsonDecoder.requireField<String>(json, 'id'),
      content: JsonDecoder.requireField<String>(json, 'content'),
      name: JsonDecoder.optionalField<String>(json, 'name'),
    );
  }

  @override
  UserMessage copyWith({
    String? id,
    String? content,
    String? name,
  }) {
    return UserMessage(
      id: id ?? this.id,
      content: content ?? this.content,
      name: name ?? this.name,
    );
  }
}

/// Tool message with tool call result.
///
/// Contains the result of a tool execution, linked to a specific tool call
/// via the [toolCallId] field.
class ToolMessage extends Message {
  @override
  final String content;
  final String toolCallId;
  final String? error;

  const ToolMessage({
    super.id,
    required this.content,
    required this.toolCallId,
    this.error,
  }) : super(role: MessageRole.tool);

  factory ToolMessage.fromJson(Map<String, dynamic> json) {
    final toolCallId = JsonDecoder.optionalField<String>(json, 'toolCallId') ??
        JsonDecoder.optionalField<String>(json, 'tool_call_id');
    
    if (toolCallId == null) {
      throw AGUIValidationError(
        message: 'Missing required field: toolCallId or tool_call_id',
        field: 'toolCallId',
        json: json,
      );
    }
    
    return ToolMessage(
      id: JsonDecoder.optionalField<String>(json, 'id'),
      content: JsonDecoder.requireField<String>(json, 'content'),
      toolCallId: toolCallId,
      error: JsonDecoder.optionalField<String>(json, 'error'),
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'toolCallId': toolCallId,
    if (error != null) 'error': error,
  };

  @override
  ToolMessage copyWith({
    String? id,
    String? content,
    String? toolCallId,
    String? error,
  }) {
    return ToolMessage(
      id: id ?? this.id,
      content: content ?? this.content,
      toolCallId: toolCallId ?? this.toolCallId,
      error: error ?? this.error,
    );
  }
}

/// Activity message embedded in a `MESSAGES_SNAPSHOT` payload.
///
/// Mirrors the canonical TypeScript `ActivityMessageSchema`
/// (`sdks/typescript/packages/core/src/types.ts`) and the Python
/// `ActivityMessage` model (`sdks/python/ag_ui/core/types.py`). The wire
/// shape is `{id, role: 'activity', activityType, content}` where
/// `content` is a JSON object (`z.record(z.any())` / `Dict[str, Any]`).
///
/// The Dart in-memory accessor for the wire `content` field is named
/// [activityContent] to avoid shadowing the parent [Message.content]
/// (which is `String?`). The wire key remains `content` in [toJson] /
/// [fromJson] for protocol parity.
class ActivityMessage extends Message {
  final String activityType;
  final Map<String, dynamic> activityContent;

  const ActivityMessage({
    required super.id,
    required this.activityType,
    required this.activityContent,
  }) : super(role: MessageRole.activity);

  factory ActivityMessage.fromJson(Map<String, dynamic> json) {
    return ActivityMessage(
      id: JsonDecoder.requireField<String>(json, 'id'),
      activityType: JsonDecoder.requireEitherField<String>(
        json,
        'activityType',
        'activity_type',
      ),
      activityContent:
          JsonDecoder.requireField<Map<String, dynamic>>(json, 'content'),
    );
  }

  @override
  Map<String, dynamic> toJson() => {
        ...super.toJson(),
        'activityType': activityType,
        'content': activityContent,
      };

  @override
  ActivityMessage copyWith({
    String? id,
    String? activityType,
    Map<String, dynamic>? activityContent,
  }) {
    return ActivityMessage(
      id: id ?? this.id,
      activityType: activityType ?? this.activityType,
      activityContent: activityContent ?? this.activityContent,
    );
  }
}

/// Reasoning message embedded in a `MESSAGES_SNAPSHOT` payload.
///
/// Mirrors the canonical TypeScript `ReasoningMessageSchema` and the
/// Python `ReasoningMessage` model. The wire shape is
/// `{id, role: 'reasoning', content, encryptedValue?}` with `content` as
/// a string and `encryptedValue` as an optional opaque cipher payload.
class ReasoningMessage extends Message {
  @override
  final String content;
  final String? encryptedValue;

  const ReasoningMessage({
    required super.id,
    required this.content,
    this.encryptedValue,
  }) : super(role: MessageRole.reasoning);

  factory ReasoningMessage.fromJson(Map<String, dynamic> json) {
    return ReasoningMessage(
      id: JsonDecoder.requireField<String>(json, 'id'),
      content: JsonDecoder.requireField<String>(json, 'content'),
      encryptedValue: JsonDecoder.optionalEitherField<String>(
        json,
        'encryptedValue',
        'encrypted_value',
      ),
    );
  }

  @override
  Map<String, dynamic> toJson() => {
        ...super.toJson(),
        if (encryptedValue != null) 'encryptedValue': encryptedValue,
      };

  @override
  ReasoningMessage copyWith({
    String? id,
    String? content,
    String? encryptedValue,
  }) {
    return ReasoningMessage(
      id: id ?? this.id,
      content: content ?? this.content,
      encryptedValue: encryptedValue ?? this.encryptedValue,
    );
  }
}