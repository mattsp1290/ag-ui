import 'dart:typed_data';

import 'package:ag_ui/ag_ui.dart';

const Object _unsetChatMessageField = Object();

enum ChatMessageType {
  user,
  assistant,
  system,
  tool,
  thinking,
  image,
  imageAttachment,
  audioAttachment,
  documentAttachment,
  reasoning,
  card,
}

class ChatMessage {
  final String id;
  final ChatMessageType type;
  final String content;
  final DateTime timestamp;
  final bool isStreaming;
  final String? toolName;
  final dynamic toolArgs;
  final dynamic toolResult;
  final String? fileName;
  final Uint8List? imageBytes;
  final Metadata? metadata;
  final String? protocolId;
  final String? subagentRunId;
  final String? subagentName;
  final String? subagentStatus;

  /// Parsed arguments for a `card` message (tool_based_generative_ui render_card),
  /// or the proposed-call args for an `approval` message (human_in_the_loop).
  final Map<String, dynamic>? cardData;

  ChatMessage({
    required this.id,
    required this.type,
    required this.content,
    required this.timestamp,
    this.isStreaming = false,
    this.toolName,
    this.toolArgs,
    this.toolResult,
    this.fileName,
    this.imageBytes,
    this.cardData,
    this.metadata,
    this.protocolId,
    this.subagentRunId,
    this.subagentName,
    this.subagentStatus,
  });

  ChatMessage copyWith({
    String? id,
    ChatMessageType? type,
    String? content,
    DateTime? timestamp,
    bool? isStreaming,
    String? toolName,
    dynamic toolArgs,
    dynamic toolResult,
    String? fileName,
    Uint8List? imageBytes,
    Map<String, dynamic>? cardData,
    Object? metadata = _unsetChatMessageField,
    Object? protocolId = _unsetChatMessageField,
    Object? subagentRunId = _unsetChatMessageField,
    Object? subagentName = _unsetChatMessageField,
    Object? subagentStatus = _unsetChatMessageField,
  }) {
    return ChatMessage(
      id: id ?? this.id,
      type: type ?? this.type,
      content: content ?? this.content,
      timestamp: timestamp ?? this.timestamp,
      isStreaming: isStreaming ?? this.isStreaming,
      toolName: toolName ?? this.toolName,
      toolArgs: toolArgs ?? this.toolArgs,
      toolResult: toolResult ?? this.toolResult,
      fileName: fileName ?? this.fileName,
      imageBytes: imageBytes ?? this.imageBytes,
      cardData: cardData ?? this.cardData,
      metadata: identical(metadata, _unsetChatMessageField)
          ? this.metadata
          : metadata as Metadata?,
      protocolId: identical(protocolId, _unsetChatMessageField)
          ? this.protocolId
          : protocolId as String?,
      subagentRunId: identical(subagentRunId, _unsetChatMessageField)
          ? this.subagentRunId
          : subagentRunId as String?,
      subagentName: identical(subagentName, _unsetChatMessageField)
          ? this.subagentName
          : subagentName as String?,
      subagentStatus: identical(subagentStatus, _unsetChatMessageField)
          ? this.subagentStatus
          : subagentStatus as String?,
    );
  }

  static ChatMessage fromUserMessage(UserMessage message) {
    return ChatMessage(
      id: message.id ?? 'user_${DateTime.now().millisecondsSinceEpoch}',
      type: ChatMessageType.user,
      content: message.content ?? '',
      timestamp: DateTime.now(),
      metadata: message.metadata,
      protocolId: message.id,
      subagentRunId: message.subagentRunId,
    );
  }

  static ChatMessage fromAssistantEvent(BaseEvent event) {
    final timestamp = event.timestamp != null
        ? DateTime.fromMillisecondsSinceEpoch(event.timestamp!)
        : DateTime.now();

    if (event is TextMessageContentEvent) {
      return ChatMessage(
        id: 'assistant_${event.timestamp ?? DateTime.now().millisecondsSinceEpoch}',
        type: ChatMessageType.assistant,
        content: event.delta,
        timestamp: timestamp,
        isStreaming: true,
        metadata: event.metadata,
        protocolId: event.messageId,
        subagentRunId: event.subagentRunId,
      );
    } else if (event is TextMessageEndEvent) {
      return ChatMessage(
        id: event.messageId,
        type: ChatMessageType.assistant,
        content: '',
        timestamp: timestamp,
        isStreaming: false,
        metadata: event.metadata,
        protocolId: event.messageId,
        subagentRunId: event.subagentRunId,
      );
    } else if (event is ToolCallResultEvent) {
      return ChatMessage(
        id: event.toolCallId,
        type: ChatMessageType.tool,
        content: 'Tool Result',
        timestamp: timestamp,
        toolName: 'Tool',
        toolResult: event.content,
        metadata: event.metadata,
        protocolId: event.toolCallId,
        subagentRunId: event.subagentRunId,
      );
    }

    return ChatMessage(
      id: 'msg_${DateTime.now().millisecondsSinceEpoch}',
      type: ChatMessageType.system,
      content: 'Unknown event type: ${event.eventType.value}',
      timestamp: timestamp,
      metadata: event.metadata,
    );
  }
}
