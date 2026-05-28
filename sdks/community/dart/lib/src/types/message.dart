/// Message types for AG-UI protocol.
///
/// This library defines the message types used in agent-user conversations,
/// including user, assistant, system, tool, and developer messages.
library;

import 'base.dart';
import 'tool.dart';

/// Role types for messages in the AG-UI protocol.
///
/// Defines the possible roles a message can have in a conversation.
enum MessageRole {
  developer('developer'),
  system('system'),
  assistant('assistant'),
  user('user'),
  tool('tool'),
  activity('activity');

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

/// User message with text or multimodal content.
///
/// Represents input from the user in the conversation. The content is a union
/// of plain text or an ordered list of multimodal parts, modeled by
/// [UserMessageContent]. Use the default constructor for text, or
/// [UserMessage.multimodal] for a list of [InputContent] parts.
class UserMessage extends Message {
  /// The user message content: [TextContent] or [MultimodalContent].
  final UserMessageContent messageContent;

  /// Creates a text user message; [content] is wrapped in [TextContent].
  ///
  /// Not `const` because it wraps [content] at runtime. For a compile-time
  /// constant, use [UserMessage.fromContent] with a `const` [TextContent].
  UserMessage({
    required super.id,
    required String content,
    super.name,
  })  : messageContent = TextContent(content),
        super(role: MessageRole.user);

  /// Creates a multimodal user message from an ordered list of [parts].
  UserMessage.multimodal({
    required super.id,
    required List<InputContent> parts,
    super.name,
  })  : messageContent = MultimodalContent(parts),
        super(role: MessageRole.user);

  /// Creates a user message from a [UserMessageContent] union value.
  const UserMessage.fromContent({
    required super.id,
    required this.messageContent,
    super.name,
  }) : super(role: MessageRole.user);

  factory UserMessage.fromJson(Map<String, dynamic> json) {
    return UserMessage.fromContent(
      id: JsonDecoder.requireField<String>(json, 'id'),
      messageContent: UserMessageContent.fromJson(json['content']),
      name: JsonDecoder.optionalField<String>(json, 'name'),
    );
  }

  /// The text of this message, or `null` when the content is multimodal.
  ///
  /// Projects [messageContent] so existing text-only readers keep working.
  @override
  String? get content => switch (messageContent) {
        TextContent(:final text) => text,
        MultimodalContent() => null,
      };

  @override
  Map<String, dynamic> toJson() => {
        if (id != null) 'id': id,
        'role': role.value,
        'content': messageContent.toJson(),
        if (name != null) 'name': name,
      };

  @override
  UserMessage copyWith({
    String? id,
    UserMessageContent? messageContent,
    String? name,
  }) {
    return UserMessage.fromContent(
      id: id ?? this.id,
      messageContent: messageContent ?? this.messageContent,
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

/// Activity message carrying structured progress state.
///
/// `activityType` identifies the shape of `content`, a free-form map of
/// activity-specific fields (e.g. `{progress: 0.5}` for an upload).
/// Emitted by the backend alongside `ACTIVITY_SNAPSHOT` / `ACTIVITY_DELTA`
/// events and included in `MESSAGES_SNAPSHOT` replays.
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
      activityType: JsonDecoder.requireField<String>(json, 'activityType'),
      activityContent:
          JsonDecoder.requireField<Map<String, dynamic>>(json, 'content'),
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    if (id != null) 'id': id,
    'role': role.value,
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

/// Reads a MIME type from JSON, accepting both `mimeType` and `mime_type`.
String? _readMimeType(Map<String, dynamic> json) =>
    JsonDecoder.optionalField<String>(json, 'mimeType') ??
    JsonDecoder.optionalField<String>(json, 'mime_type');

/// The source of a multimodal [InputContent] part.
///
/// A discriminated union on `type`: [DataSource] (inline data, e.g. base64)
/// or [UrlSource] (a remote URL). Use [InputContentSource.fromJson] to decode.
sealed class InputContentSource extends AGUIModel {
  const InputContentSource();

  /// The source discriminator: `data` or `url`.
  String get sourceType;

  /// Decodes an [InputContentSource] from JSON, dispatching on `type`.
  factory InputContentSource.fromJson(Map<String, dynamic> json) {
    final type = JsonDecoder.requireField<String>(json, 'type');
    switch (type) {
      case 'data':
        return DataSource.fromJson(json);
      case 'url':
        return UrlSource.fromJson(json);
      default:
        throw AGUIValidationError(
          message: 'Invalid input content source type: $type',
          field: 'type',
          value: type,
          json: json,
        );
    }
  }
}

/// Inline content source carrying a data payload (e.g. base64-encoded bytes).
///
/// [mimeType] is required for data sources.
class DataSource extends InputContentSource {
  /// The inline data payload, typically base64-encoded.
  final String value;

  /// The MIME type of [value]. Required.
  final String mimeType;

  const DataSource({required this.value, required this.mimeType});

  @override
  String get sourceType => 'data';

  factory DataSource.fromJson(Map<String, dynamic> json) {
    final mimeType = _readMimeType(json);
    if (mimeType == null) {
      throw AGUIValidationError(
        message: 'DataSource requires a mimeType',
        field: 'mimeType',
        json: json,
      );
    }
    return DataSource(
      value: JsonDecoder.requireField<String>(json, 'value'),
      mimeType: mimeType,
    );
  }

  @override
  Map<String, dynamic> toJson() => {
        'type': sourceType,
        'value': value,
        'mimeType': mimeType,
      };

  @override
  DataSource copyWith({String? value, String? mimeType}) => DataSource(
        value: value ?? this.value,
        mimeType: mimeType ?? this.mimeType,
      );
}

/// Remote content source referenced by URL.
///
/// [mimeType] is optional for URL sources.
class UrlSource extends InputContentSource {
  /// The URL of the content.
  final String value;

  /// The optional MIME type of the referenced content.
  final String? mimeType;

  const UrlSource({required this.value, this.mimeType});

  @override
  String get sourceType => 'url';

  factory UrlSource.fromJson(Map<String, dynamic> json) => UrlSource(
        value: JsonDecoder.requireField<String>(json, 'value'),
        mimeType: _readMimeType(json),
      );

  @override
  Map<String, dynamic> toJson() => {
        'type': sourceType,
        'value': value,
        if (mimeType != null) 'mimeType': mimeType,
      };

  @override
  UrlSource copyWith({String? value, String? mimeType}) => UrlSource(
        value: value ?? this.value,
        mimeType: mimeType ?? this.mimeType,
      );
}

/// Parses the shared `source` (+ optional `metadata`) of a media input part.
({InputContentSource source, Object? metadata}) _parseMediaPart(
  Map<String, dynamic> json,
  String type,
) {
  final rawSource = json['source'];
  if (rawSource is! Map<String, dynamic>) {
    throw AGUIValidationError(
      message: '$type input content requires a source object',
      field: 'source',
      value: rawSource,
      json: json,
    );
  }
  return (
    source: InputContentSource.fromJson(rawSource),
    metadata: json['metadata'] as Object?,
  );
}

/// Serializes the shared shape of a media input part.
Map<String, dynamic> _mediaToJson(
  String type,
  InputContentSource source,
  Object? metadata,
) => {
      'type': type,
      'source': source.toJson(),
      if (metadata != null) 'metadata': metadata,
    };

/// A single typed part of a multimodal [UserMessage].
///
/// A discriminated union on `type`: [TextInputContent], [ImageInputContent],
/// [AudioInputContent], [VideoInputContent], [DocumentInputContent], or the
/// legacy [BinaryInputContent]. Use [InputContent.fromJson] to decode.
sealed class InputContent extends AGUIModel with TypeDiscriminator {
  const InputContent();

  /// Decodes an [InputContent] from JSON, dispatching on `type`.
  factory InputContent.fromJson(Map<String, dynamic> json) {
    final type = JsonDecoder.requireField<String>(json, 'type');
    switch (type) {
      case 'text':
        return TextInputContent.fromJson(json);
      case 'image':
        return ImageInputContent.fromJson(json);
      case 'audio':
        return AudioInputContent.fromJson(json);
      case 'video':
        return VideoInputContent.fromJson(json);
      case 'document':
        return DocumentInputContent.fromJson(json);
      case 'binary':
        return BinaryInputContent.fromJson(json);
      default:
        throw AGUIValidationError(
          message: 'Invalid input content type: $type',
          field: 'type',
          value: type,
          json: json,
        );
    }
  }
}

/// Plain text part of a multimodal message.
class TextInputContent extends InputContent {
  /// The text payload.
  final String text;

  const TextInputContent(this.text);

  @override
  String get type => 'text';

  factory TextInputContent.fromJson(Map<String, dynamic> json) =>
      TextInputContent(JsonDecoder.requireField<String>(json, 'text'));

  @override
  Map<String, dynamic> toJson() => {'type': type, 'text': text};

  @override
  TextInputContent copyWith({String? text}) =>
      TextInputContent(text ?? this.text);
}

/// Image part of a multimodal message.
class ImageInputContent extends InputContent {
  /// The image source (data or URL).
  final InputContentSource source;

  /// Free-form, provider-specific metadata. Serialized only when non-null.
  final Object? metadata;

  const ImageInputContent({required this.source, this.metadata});

  @override
  String get type => 'image';

  factory ImageInputContent.fromJson(Map<String, dynamic> json) {
    final parsed = _parseMediaPart(json, 'image');
    return ImageInputContent(source: parsed.source, metadata: parsed.metadata);
  }

  @override
  Map<String, dynamic> toJson() => _mediaToJson(type, source, metadata);

  @override
  ImageInputContent copyWith({InputContentSource? source, Object? metadata}) =>
      ImageInputContent(
        source: source ?? this.source,
        metadata: metadata ?? this.metadata,
      );
}

/// Audio part of a multimodal message.
class AudioInputContent extends InputContent {
  /// The audio source (data or URL).
  final InputContentSource source;

  /// Free-form, provider-specific metadata. Serialized only when non-null.
  final Object? metadata;

  const AudioInputContent({required this.source, this.metadata});

  @override
  String get type => 'audio';

  factory AudioInputContent.fromJson(Map<String, dynamic> json) {
    final parsed = _parseMediaPart(json, 'audio');
    return AudioInputContent(source: parsed.source, metadata: parsed.metadata);
  }

  @override
  Map<String, dynamic> toJson() => _mediaToJson(type, source, metadata);

  @override
  AudioInputContent copyWith({InputContentSource? source, Object? metadata}) =>
      AudioInputContent(
        source: source ?? this.source,
        metadata: metadata ?? this.metadata,
      );
}

/// Video part of a multimodal message.
class VideoInputContent extends InputContent {
  /// The video source (data or URL).
  final InputContentSource source;

  /// Free-form, provider-specific metadata. Serialized only when non-null.
  final Object? metadata;

  const VideoInputContent({required this.source, this.metadata});

  @override
  String get type => 'video';

  factory VideoInputContent.fromJson(Map<String, dynamic> json) {
    final parsed = _parseMediaPart(json, 'video');
    return VideoInputContent(source: parsed.source, metadata: parsed.metadata);
  }

  @override
  Map<String, dynamic> toJson() => _mediaToJson(type, source, metadata);

  @override
  VideoInputContent copyWith({InputContentSource? source, Object? metadata}) =>
      VideoInputContent(
        source: source ?? this.source,
        metadata: metadata ?? this.metadata,
      );
}

/// Document part of a multimodal message.
class DocumentInputContent extends InputContent {
  /// The document source (data or URL).
  final InputContentSource source;

  /// Free-form, provider-specific metadata. Serialized only when non-null.
  final Object? metadata;

  const DocumentInputContent({required this.source, this.metadata});

  @override
  String get type => 'document';

  factory DocumentInputContent.fromJson(Map<String, dynamic> json) {
    final parsed = _parseMediaPart(json, 'document');
    return DocumentInputContent(
      source: parsed.source,
      metadata: parsed.metadata,
    );
  }

  @override
  Map<String, dynamic> toJson() => _mediaToJson(type, source, metadata);

  @override
  DocumentInputContent copyWith({
    InputContentSource? source,
    Object? metadata,
  }) =>
      DocumentInputContent(
        source: source ?? this.source,
        metadata: metadata ?? this.metadata,
      );
}

/// Legacy binary content part.
///
/// Requires a non-empty [mimeType] and at least one of [id], [url], or [data].
class BinaryInputContent extends InputContent {
  /// The MIME type of the binary payload. Required and non-empty.
  final String mimeType;

  /// An opaque identifier for previously-uploaded content.
  final String? id;

  /// A URL referencing the content.
  final String? url;

  /// An inline data payload (e.g. base64-encoded).
  final String? data;

  /// An optional display filename.
  final String? filename;

  const BinaryInputContent({
    required this.mimeType,
    this.id,
    this.url,
    this.data,
    this.filename,
  })  : assert(mimeType != '', 'BinaryInputContent requires a non-empty mimeType'),
        assert(
          id != null || url != null || data != null,
          'BinaryInputContent requires at least one of id, url, or data',
        );

  @override
  String get type => 'binary';

  factory BinaryInputContent.fromJson(Map<String, dynamic> json) {
    final mimeType = _readMimeType(json);
    if (mimeType == null || mimeType.isEmpty) {
      throw AGUIValidationError(
        message: 'BinaryInputContent requires a non-empty mimeType',
        field: 'mimeType',
        json: json,
      );
    }
    final id = JsonDecoder.optionalField<String>(json, 'id');
    final url = JsonDecoder.optionalField<String>(json, 'url');
    final data = JsonDecoder.optionalField<String>(json, 'data');
    if (id == null && url == null && data == null) {
      throw AGUIValidationError(
        message: 'BinaryInputContent requires at least one of id, url, or data',
        field: 'id',
        json: json,
      );
    }
    return BinaryInputContent(
      mimeType: mimeType,
      id: id,
      url: url,
      data: data,
      filename: JsonDecoder.optionalField<String>(json, 'filename'),
    );
  }

  @override
  Map<String, dynamic> toJson() => {
        'type': type,
        'mimeType': mimeType,
        if (id != null) 'id': id,
        if (url != null) 'url': url,
        if (data != null) 'data': data,
        if (filename != null) 'filename': filename,
      };

  @override
  BinaryInputContent copyWith({
    String? mimeType,
    String? id,
    String? url,
    String? data,
    String? filename,
  }) =>
      BinaryInputContent(
        mimeType: mimeType ?? this.mimeType,
        id: id ?? this.id,
        url: url ?? this.url,
        data: data ?? this.data,
        filename: filename ?? this.filename,
      );
}

/// The content union for a [UserMessage]: plain text or multimodal parts.
///
/// Mirrors the canonical `string | InputContent[]` shape. [toJson] returns a
/// `String` for [TextContent] or a `List` for [MultimodalContent].
sealed class UserMessageContent {
  const UserMessageContent();

  /// Serializes to a JSON `String` (text) or `List` (multimodal parts).
  Object toJson();

  /// Decodes from a raw `content` value: a `String` or a `List` of parts.
  factory UserMessageContent.fromJson(Object? raw) {
    if (raw is String) {
      return TextContent(raw);
    }
    if (raw is List) {
      final parts = <InputContent>[];
      for (var i = 0; i < raw.length; i++) {
        final item = raw[i];
        if (item is! Map<String, dynamic>) {
          throw AGUIValidationError(
            message: 'UserMessage content part at index $i must be an object',
            field: 'content[$i]',
            value: item,
          );
        }
        try {
          parts.add(InputContent.fromJson(item));
        } on AGUIValidationError catch (e) {
          throw AGUIValidationError(
            message: 'Invalid content part at index $i: ${e.message}',
            field: 'content[$i]',
            value: item,
          );
        }
      }
      return MultimodalContent(parts);
    }
    throw AGUIValidationError(
      message: 'UserMessage content must be a String or a List of parts',
      field: 'content',
      value: raw,
    );
  }
}

/// Plain-text user message content. Serializes to a JSON `String`.
class TextContent extends UserMessageContent {
  /// The text payload.
  final String text;

  const TextContent(this.text);

  @override
  String toJson() => text;
}

/// Multimodal user message content. Serializes to a JSON `List`.
class MultimodalContent extends UserMessageContent {
  /// The ordered list of content parts.
  final List<InputContent> parts;

  const MultimodalContent(this.parts);

  @override
  List<Map<String, dynamic>> toJson() =>
      parts.map((part) => part.toJson()).toList();
}