part of 'agui_event_projection.dart';

class _AgUiProjectionStore {
  final String? Function(String?) _subagentName;
  final String? Function(String?) _subagentStatus;

  final List<ChatMessage> messages = [];
  final Map<(int, String?, String, ChatMessageType), String>
  _messageDisplayIds = {};
  final Map<(int, String?, String), String> _toolDisplayIds = {};
  final Map<String?, String> activeTextMessageIds = {};
  final Map<String?, String> activeReasoningMessageIds = {};
  final Map<String?, String> activeToolCallIds = {};
  int _runScope = 0;

  _AgUiProjectionStore({
    required String? Function(String?) subagentName,
    required String? Function(String?) subagentStatus,
  }) : _subagentName = subagentName,
       _subagentStatus = subagentStatus;

  void beginRun(
    Iterable<String> resumedSubagentIds, {
    bool preserveAll = false,
  }) {
    final resumed = resumedSubagentIds.toSet();
    final previousScope = _runScope;
    final resumedMessages = <(int, String?, String, ChatMessageType), String>{};
    final resumedTools = <(int, String?, String), String>{};
    _runScope++;

    for (final entry in _messageDisplayIds.entries) {
      final (scope, subagentRunId, protocolId, type) = entry.key;
      if (scope == previousScope &&
          (preserveAll || resumed.contains(subagentRunId))) {
        resumedMessages[(_runScope, subagentRunId, protocolId, type)] =
            entry.value;
      }
    }
    for (final entry in _toolDisplayIds.entries) {
      final (scope, subagentRunId, protocolId) = entry.key;
      if (scope == previousScope &&
          (preserveAll || resumed.contains(subagentRunId))) {
        resumedTools[(_runScope, subagentRunId, protocolId)] = entry.value;
      }
    }
    _messageDisplayIds
      ..clear()
      ..addAll(resumedMessages);
    _toolDisplayIds
      ..clear()
      ..addAll(resumedTools);
  }

  void addReasoningMessage(String text, {String? id}) {
    if (text.isEmpty) return;
    upsert(
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
    activeTextMessageIds.clear();
    activeReasoningMessageIds.clear();
    activeToolCallIds.clear();
  }

  void startReasoning(
    String messageId,
    String? subagentRunId,
    DateTime timestamp,
    Metadata? metadata,
  ) {
    final projectedId = displayId(
      subagentRunId,
      messageId,
      ChatMessageType.reasoning,
    );
    activeReasoningMessageIds[subagentRunId] = projectedId;
    upsert(
      ChatMessage(
        id: projectedId,
        type: ChatMessageType.reasoning,
        content: '',
        timestamp: timestamp,
        isStreaming: true,
        metadata: metadata,
        protocolId: messageId,
        subagentRunId: subagentRunId,
        subagentName: _subagentName(subagentRunId),
        subagentStatus: _subagentStatus(subagentRunId),
      ),
    );
  }

  void appendText(
    String protocolId,
    String? subagentRunId,
    String delta, {
    Metadata? metadata,
    bool create = false,
  }) {
    appendDisplay(
      displayId(subagentRunId, protocolId, ChatMessageType.assistant),
      ChatMessageType.assistant,
      delta,
      metadata: metadata,
      protocolId: protocolId,
      subagentRunId: subagentRunId,
      create: create,
    );
  }

  void appendReasoning(
    String protocolId,
    String? subagentRunId,
    String delta, {
    Metadata? metadata,
    bool create = false,
  }) {
    appendDisplay(
      displayId(subagentRunId, protocolId, ChatMessageType.reasoning),
      ChatMessageType.reasoning,
      delta,
      protocolId: protocolId,
      metadata: metadata,
      subagentRunId: subagentRunId,
      create: create,
    );
  }

  void appendDisplay(
    String displayId,
    ChatMessageType type,
    String delta, {
    String? protocolId,
    Metadata? metadata,
    String? subagentRunId,
    bool create = false,
  }) {
    final index = type == ChatMessageType.assistant
        ? textMessageIndex(displayId)
        : messageIndex(displayId, type);
    if (index < 0) {
      if (!create) return;
      upsert(
        ChatMessage(
          id: displayId,
          type: type,
          content: delta,
          timestamp: DateTime.now(),
          isStreaming: true,
          metadata: metadata,
          protocolId: protocolId,
          subagentRunId: subagentRunId,
          subagentName: _subagentName(subagentRunId),
          subagentStatus: _subagentStatus(subagentRunId),
        ),
      );
      return;
    }
    final previous = messages[index];
    messages[index] = previous.copyWith(
      content: previous.content + delta,
      isStreaming: true,
      metadata: mergeMetadata(previous.metadata, metadata),
      subagentName: _subagentName(subagentRunId) ?? previous.subagentName,
      subagentStatus: _subagentStatus(subagentRunId) ?? previous.subagentStatus,
    );
  }

  void finishMessage(
    String protocolId,
    String? subagentRunId,
    ChatMessageType type, {
    Metadata? metadata,
  }) {
    final projectedId = displayId(subagentRunId, protocolId, type);
    final index = type == ChatMessageType.assistant
        ? textMessageIndex(projectedId)
        : messageIndex(projectedId, type);
    if (index < 0) return;
    final previous = messages[index];
    messages[index] = previous.copyWith(
      isStreaming: false,
      metadata: mergeMetadata(previous.metadata, metadata),
    );
  }

  void projectTool(
    String protocolId,
    String? subagentRunId, {
    String? name,
    String? args,
    String? argsDelta,
    String? result,
    Metadata? metadata,
    bool? isStreaming,
  }) {
    final displayId = toolDisplayId(subagentRunId, protocolId);
    final index = messageIndex(displayId, ChatMessageType.tool);
    final previous = index < 0 ? null : messages[index];
    final toolName = name ?? previous?.toolName ?? 'Tool';
    final toolArgs = args ?? '${previous?.toolArgs ?? ''}${argsDelta ?? ''}';
    final toolResult = result ?? previous?.toolResult;
    upsert(
      ChatMessage(
        id: displayId,
        type: ChatMessageType.tool,
        content: [
          '🔧 Tool: $toolName',
          if (toolArgs.isNotEmpty) 'Arguments: $toolArgs',
          if (toolResult != null) 'Result: $toolResult',
        ].join('\n'),
        timestamp: previous?.timestamp ?? DateTime.now(),
        isStreaming: isStreaming ?? previous?.isStreaming ?? false,
        toolName: toolName,
        toolArgs: toolArgs,
        toolResult: toolResult,
        metadata: mergeMetadata(previous?.metadata, metadata),
        protocolId: protocolId,
        subagentRunId: subagentRunId,
        subagentName: _subagentName(subagentRunId),
        subagentStatus: _subagentStatus(subagentRunId),
      ),
    );
  }

  void refreshSubagentMessages(String id, AgUiSubagentState state) {
    final currentIds = <String>{
      for (final entry in _messageDisplayIds.entries)
        if (entry.key.$1 == _runScope && entry.key.$2 == id) entry.value,
      for (final entry in _toolDisplayIds.entries)
        if (entry.key.$1 == _runScope && entry.key.$2 == id) entry.value,
    };
    for (var i = 0; i < messages.length; i++) {
      final message = messages[i];
      if (!currentIds.contains(message.id)) continue;
      messages[i] = message.copyWith(
        metadata: mergeMetadata(message.metadata, state.metadata),
        subagentName: state.name,
        subagentStatus: state.status.label,
      );
    }
  }

  String displayId(
    String? subagentRunId,
    String protocolId,
    ChatMessageType type,
  ) {
    final key = (_runScope, subagentRunId, protocolId, type);
    return _messageDisplayIds.putIfAbsent(key, () {
      if (subagentRunId != null) return uid('subagent-message');
      return messageIndex(protocolId, type) < 0 ? protocolId : uid('message');
    });
  }

  String toolDisplayId(String? subagentRunId, String protocolId) {
    final key = (_runScope, subagentRunId, protocolId);
    return _toolDisplayIds.putIfAbsent(key, () {
      if (subagentRunId != null) return uid('subagent-tool');
      return messageIndex(protocolId, ChatMessageType.tool) < 0
          ? protocolId
          : uid('tool');
    });
  }

  void upsert(ChatMessage message) {
    final index = messageIndex(message.id, message.type);
    if (index < 0) {
      messages.add(message);
      return;
    }
    final previous = messages[index];
    messages[index] = message.copyWith(
      metadata: mergeMetadata(previous.metadata, message.metadata),
      subagentName: message.subagentName ?? previous.subagentName,
      subagentStatus: message.subagentStatus ?? previous.subagentStatus,
    );
  }

  int messageIndex(String displayId, ChatMessageType type) => messages
      .indexWhere((message) => message.id == displayId && message.type == type);

  int textMessageIndex(String displayId) => messages.indexWhere(
    (message) =>
        message.id == displayId &&
        (message.type == ChatMessageType.assistant ||
            message.type == ChatMessageType.user ||
            message.type == ChatMessageType.system),
  );
}
