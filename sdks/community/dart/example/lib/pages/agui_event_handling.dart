import 'package:ag_ui/ag_ui.dart';
import 'package:flutter/foundation.dart';

import '../models/chat_message.dart';
import '../services/ids.dart';

mixin AgUiEventHandling on ChangeNotifier {
  bool disposed = false;
  bool _terminal = false;
  final List<ChatMessage> messages = [];
  String? _textMessageId;

  bool get runIsTerminal => _terminal;

  void onRunReset() {}

  void beginRun() {
    _terminal = false;
    finishStreaming();
  }

  /// Handles shared projections. Snapshots deliberately return false so hosts can
  /// reconcile authoritative tool/state history before calling [reconcileSnapshot].
  bool handleCommonEvent(BaseEvent event) {
    if (_terminal) return true;
    if (event is TextMessageStartEvent) {
      _textMessageId = event.messageId;
      _upsert(
        ChatMessage(
          id: event.messageId,
          type: ChatMessageType.assistant,
          content: '',
          timestamp: _timestamp(event),
          isStreaming: true,
        ),
      );
      return true;
    }
    if (event is TextMessageContentEvent) {
      _append(event.messageId, ChatMessageType.assistant, event.delta);
      return true;
    }
    if (event is TextMessageChunkEvent) {
      final id = event.messageId ?? _textMessageId ?? uid('assistant');
      _textMessageId = id;
      _append(id, ChatMessageType.assistant, event.delta ?? '', create: true);
      return true;
    }
    if (event is TextMessageEndEvent) {
      _finish(event.messageId);
      _textMessageId = null;
      return true;
    }
    if (event is ReasoningMessageStartEvent) {
      _upsert(
        ChatMessage(
          id: event.messageId,
          type: ChatMessageType.reasoning,
          content: '',
          timestamp: _timestamp(event),
          isStreaming: true,
        ),
      );
      return true;
    }
    if (event is ReasoningStartEvent) {
      // REASONING_START identifies the enclosing block. Use it unless the
      // canonical message-start event supplies the message identity afterward.
      _upsert(
        ChatMessage(
          id: event.messageId,
          type: ChatMessageType.reasoning,
          content: '',
          timestamp: _timestamp(event),
          isStreaming: true,
        ),
      );
      return true;
    }
    if (event is ReasoningMessageContentEvent) {
      _append(
        event.messageId,
        ChatMessageType.reasoning,
        event.delta,
        create: true,
      );
      return true;
    }
    if (event is ReasoningMessageEndEvent) {
      _finish(event.messageId);
      return true;
    }
    if (event is ReasoningEndEvent) {
      _finish(event.messageId);
      return true;
    }
    if (event is RunErrorEvent) {
      finishStreaming();
      messages.add(
        ChatMessage(
          id: uid('error'),
          type: ChatMessageType.system,
          content: '⚠️ Run error: ${event.message}',
          timestamp: _timestamp(event),
        ),
      );
      _terminal = true;
      onRunReset();
      return true;
    }
    if (event is RunFinishedEvent) {
      finishStreaming();
      _terminal = true;
      return false;
    }
    return false;
  }

  void reconcileSnapshot(List<Message> snapshot) {
    for (final message in snapshot) {
      final id = message.id;
      if (id == null || id.isEmpty) continue;
      if (message is AssistantMessage) {
        _upsert(
          ChatMessage(
            id: id,
            type: ChatMessageType.assistant,
            content: message.content ?? '',
            timestamp: DateTime.now(),
          ),
        );
      } else if (message is ReasoningMessage) {
        final content = message.content ?? message.thinking ?? '';
        if (content.isNotEmpty) {
          _upsert(
            ChatMessage(
              id: id,
              type: ChatMessageType.reasoning,
              content: content,
              timestamp: DateTime.now(),
            ),
          );
        }
      }
    }
  }

  void addReasoningMessage(String text, {String? id}) {
    if (text.isEmpty) return;
    _upsert(
      ChatMessage(
        id: id ?? uid('reasoning'),
        type: ChatMessageType.reasoning,
        content: text,
        timestamp: DateTime.now(),
      ),
    );
  }

  void finishStreaming() {
    for (var i = 0; i < messages.length; i++) {
      if (messages[i].isStreaming) {
        messages[i] = messages[i].copyWith(isStreaming: false);
      }
    }
    _textMessageId = null;
  }

  void _append(
    String id,
    ChatMessageType type,
    String delta, {
    bool create = false,
  }) {
    final index = messages.indexWhere((message) => message.id == id);
    if (index < 0) {
      if (create) {
        _upsert(
          ChatMessage(
            id: id,
            type: type,
            content: delta,
            timestamp: DateTime.now(),
            isStreaming: true,
          ),
        );
      }
      return;
    }
    messages[index] = messages[index].copyWith(
      content: messages[index].content + delta,
      isStreaming: true,
    );
  }

  void _finish(String? id) {
    if (id == null) return;
    final index = messages.indexWhere((message) => message.id == id);
    if (index >= 0) {
      messages[index] = messages[index].copyWith(isStreaming: false);
    }
  }

  void _upsert(ChatMessage message) {
    final index = messages.indexWhere((item) => item.id == message.id);
    if (index < 0) {
      messages.add(message);
    } else {
      messages[index] = message;
    }
  }

  DateTime _timestamp(BaseEvent event) => event.timestamp == null
      ? DateTime.now()
      : DateTime.fromMillisecondsSinceEpoch(event.timestamp!);
}
