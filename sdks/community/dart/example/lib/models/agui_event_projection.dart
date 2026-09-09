import 'package:ag_ui/ag_ui.dart';
import 'chat_message.dart';
import '../services/ids.dart';

const Object _unsetProjectionField = Object();

enum AgUiRunStatus { idle, running, completed, awaitingInput, error }

enum AgUiSubagentStatus {
  running,
  completed,
  suspended,
  failed;

  String get label => name;
}

/// The lifecycle state projected for one subagent in the current root run.
class AgUiSubagentState {
  final String subagentRunId;
  final String? name;
  final String? description;
  final String? parentSubagentRunId;
  final String? parentToolCallId;
  final String? parentMessageId;
  final AgUiSubagentStatus status;
  final dynamic result;
  final List<String>? interruptIds;
  final Metadata? metadata;

  const AgUiSubagentState({
    required this.subagentRunId,
    this.name,
    this.description,
    this.parentSubagentRunId,
    this.parentToolCallId,
    this.parentMessageId,
    this.status = AgUiSubagentStatus.running,
    this.result,
    this.interruptIds,
    this.metadata,
  });

  AgUiSubagentState copyWith({
    Object? name = _unsetProjectionField,
    Object? description = _unsetProjectionField,
    Object? parentSubagentRunId = _unsetProjectionField,
    Object? parentToolCallId = _unsetProjectionField,
    Object? parentMessageId = _unsetProjectionField,
    AgUiSubagentStatus? status,
    Object? result = _unsetProjectionField,
    Object? interruptIds = _unsetProjectionField,
    Object? metadata = _unsetProjectionField,
  }) {
    return AgUiSubagentState(
      subagentRunId: subagentRunId,
      name: identical(name, _unsetProjectionField)
          ? this.name
          : name as String?,
      description: identical(description, _unsetProjectionField)
          ? this.description
          : description as String?,
      parentSubagentRunId: identical(parentSubagentRunId, _unsetProjectionField)
          ? this.parentSubagentRunId
          : parentSubagentRunId as String?,
      parentToolCallId: identical(parentToolCallId, _unsetProjectionField)
          ? this.parentToolCallId
          : parentToolCallId as String?,
      parentMessageId: identical(parentMessageId, _unsetProjectionField)
          ? this.parentMessageId
          : parentMessageId as String?,
      status: status ?? this.status,
      result: identical(result, _unsetProjectionField) ? this.result : result,
      interruptIds: identical(interruptIds, _unsetProjectionField)
          ? this.interruptIds
          : interruptIds as List<String>?,
      metadata: identical(metadata, _unsetProjectionField)
          ? this.metadata
          : metadata as Metadata?,
    );
  }
}

class AgUiEventProjection {
  bool _terminal = false;
  final List<ChatMessage> messages = [];

  final Map<String, AgUiSubagentState> _subagents = {};
  final Map<(String?, String, ChatMessageType), String> _messageDisplayIds = {};
  final Map<(String?, String), String> _toolDisplayIds = {};
  final Map<String?, String> _activeTextMessageIds = {};
  final Map<String?, String> _activeReasoningMessageIds = {};
  final Map<String?, String> _activeToolCallIds = {};
  List<TokenUsage> _runUsage = const [];
  RunFinishedOutcome? _runOutcome;
  AgUiRunStatus _runStatus = AgUiRunStatus.idle;

  bool get runIsTerminal => _terminal;
  AgUiRunStatus get runStatus => _runStatus;
  bool get runIsAwaitingInput => _runStatus == AgUiRunStatus.awaitingInput;
  RunFinishedOutcome? get runOutcome => _runOutcome;
  List<TokenUsage> get runUsage => List.unmodifiable(_runUsage);
  Map<String, AgUiSubagentState> get subagents => Map.unmodifiable(_subagents);

  final void Function()? onRunError;

  AgUiEventProjection({this.onRunError});

  void beginRun({Iterable<String> resumedSubagentIds = const []}) {
    final resumed = <String, AgUiSubagentState>{
      for (final id in resumedSubagentIds)
        if (_subagents[id] case final state?)
          id: state.copyWith(
            status: AgUiSubagentStatus.running,
            result: null,
            interruptIds: null,
          ),
    };
    _terminal = false;
    _runStatus = AgUiRunStatus.running;
    _runOutcome = null;
    _runUsage = const [];
    _subagents.clear();
    _subagents.addAll(resumed);
    for (final entry in resumed.entries) {
      _refreshSubagentMessages(entry.key, entry.value);
    }
    _activeTextMessageIds.clear();
    _activeReasoningMessageIds.clear();
    _activeToolCallIds.clear();
    finishStreaming();
  }

  /// Handles shared projections. Snapshots deliberately return false so hosts can
  /// reconcile authoritative tool/state history before calling [reconcileSnapshot].
  bool handleEvent(BaseEvent event) {
    if (_terminal) return true;
    for (final handler in [
      _handleRunLifecycle,
      _handleSubagentLifecycle,
      _handleTextEvent,
      _handleReasoningEvent,
      _handleToolEvent,
    ]) {
      final handled = handler(event);
      if (handled != null) return handled;
    }
    return false;
  }

  bool? _handleRunLifecycle(BaseEvent event) {
    if (event is RunStartedEvent) {
      _runStatus = AgUiRunStatus.running;
      return false;
    }
    if (event is RunErrorEvent) {
      finishStreaming();
      _runUsage = List.unmodifiable(event.usage ?? const []);
      _runOutcome = null;
      _runStatus = AgUiRunStatus.error;
      messages.add(
        ChatMessage(
          id: uid('error'),
          type: ChatMessageType.system,
          content: '⚠️ Run error: ${event.message}',
          timestamp: _timestamp(event),
          metadata: event.metadata,
        ),
      );
      _terminal = true;
      onRunError?.call();
      return true;
    }
    if (event is RunFinishedEvent) {
      finishStreaming();
      _runUsage = List.unmodifiable(event.usage ?? const []);
      _runOutcome = event.outcome;
      _runStatus = event.outcome is RunFinishedInterruptOutcome
          ? AgUiRunStatus.awaitingInput
          : AgUiRunStatus.completed;
      messages.add(
        ChatMessage(
          id: uid('run-status'),
          type: ChatMessageType.system,
          content: runIsAwaitingInput
              ? '⏸️ Run awaiting input'
              : '✅ Run completed',
          timestamp: _timestamp(event),
          metadata: event.metadata,
        ),
      );
      _terminal = true;
      return false;
    }
    return null;
  }

  bool? _handleSubagentLifecycle(BaseEvent event) {
    if (event is SubagentStartedEvent) {
      _setSubagent(
        event.subagentRunId,
        name: event.name,
        description: event.description,
        parentSubagentRunId: event.parentSubagentRunId,
        parentToolCallId: event.parentToolCallId,
        parentMessageId: event.parentMessageId,
        status: AgUiSubagentStatus.running,
        metadata: event.metadata,
      );
      return true;
    }
    if (event is SubagentFinishedEvent) {
      final suspended = event.outcome is SubagentFinishedSuspendedOutcome;
      final outcome = event.outcome;
      _setSubagent(
        event.subagentRunId,
        status: suspended
            ? AgUiSubagentStatus.suspended
            : AgUiSubagentStatus.completed,
        result: event.result,
        interruptIds: outcome is SubagentFinishedSuspendedOutcome
            ? outcome.interruptIds
            : null,
        metadata: event.metadata,
      );
      return true;
    }
    if (event is SubagentErrorEvent) {
      _setSubagent(
        event.subagentRunId,
        status: AgUiSubagentStatus.failed,
        metadata: event.metadata,
      );
      return true;
    }

    return null;
  }

  bool? _handleTextEvent(BaseEvent event) {
    if (event is TextMessageStartEvent) {
      final displayId = _displayId(
        event.subagentRunId,
        event.messageId,
        ChatMessageType.assistant,
      );
      _activeTextMessageIds[event.subagentRunId] = displayId;
      _upsert(
        ChatMessage(
          id: displayId,
          type: _textType(event.role),
          content: '',
          timestamp: _timestamp(event),
          isStreaming: true,
          metadata: event.metadata,
          protocolId: event.messageId,
          subagentRunId: event.subagentRunId,
          subagentName: _subagentName(event.subagentRunId),
          subagentStatus: _subagentStatus(event.subagentRunId),
        ),
      );
      return true;
    }
    if (event is TextMessageContentEvent) {
      _appendText(
        event.messageId,
        event.subagentRunId,
        event.delta,
        metadata: event.metadata,
        create: true,
      );
      return true;
    }
    if (event is TextMessageChunkEvent) {
      final displayId = event.messageId == null
          ? (_activeTextMessageIds[event.subagentRunId] ??
                _displayId(
                  event.subagentRunId,
                  uid('assistant'),
                  ChatMessageType.assistant,
                ))
          : _displayId(
              event.subagentRunId,
              event.messageId!,
              ChatMessageType.assistant,
            );
      _activeTextMessageIds[event.subagentRunId] = displayId;
      _appendDisplay(
        displayId,
        _textType(event.role),
        event.delta ?? '',
        protocolId: event.messageId,
        metadata: event.metadata,
        subagentRunId: event.subagentRunId,
        create: true,
      );
      return true;
    }
    if (event is TextMessageEndEvent) {
      _finishMessage(
        event.messageId,
        event.subagentRunId,
        ChatMessageType.assistant,
        metadata: event.metadata,
      );
      final displayId = _displayId(
        event.subagentRunId,
        event.messageId,
        ChatMessageType.assistant,
      );
      if (_activeTextMessageIds[event.subagentRunId] == displayId) {
        _activeTextMessageIds.remove(event.subagentRunId);
      }
      return true;
    }

    return null;
  }

  bool? _handleReasoningEvent(BaseEvent event) {
    if (event is ReasoningMessageStartEvent) {
      _startReasoning(
        event.messageId,
        event.subagentRunId,
        _timestamp(event),
        event.metadata,
      );
      return true;
    }
    if (event is ReasoningStartEvent) {
      _startReasoning(
        event.messageId,
        event.subagentRunId,
        _timestamp(event),
        event.metadata,
      );
      return true;
    }
    if (event is ReasoningMessageContentEvent) {
      _appendReasoning(
        event.messageId,
        event.subagentRunId,
        event.delta,
        metadata: event.metadata,
        create: true,
      );
      return true;
    }
    if (event is ReasoningMessageChunkEvent) {
      final displayId = event.messageId == null
          ? (_activeReasoningMessageIds[event.subagentRunId] ??
                _displayId(
                  event.subagentRunId,
                  uid('reasoning'),
                  ChatMessageType.reasoning,
                ))
          : _displayId(
              event.subagentRunId,
              event.messageId!,
              ChatMessageType.reasoning,
            );
      _activeReasoningMessageIds[event.subagentRunId] = displayId;
      _appendDisplay(
        displayId,
        ChatMessageType.reasoning,
        event.delta ?? '',
        metadata: event.metadata,
        subagentRunId: event.subagentRunId,
        create: true,
      );
      return true;
    }
    if (event is ReasoningMessageEndEvent) {
      _finishMessage(
        event.messageId,
        event.subagentRunId,
        ChatMessageType.reasoning,
        metadata: event.metadata,
      );
      return true;
    }
    if (event is ReasoningEndEvent) {
      _finishMessage(
        event.messageId,
        event.subagentRunId,
        ChatMessageType.reasoning,
        metadata: event.metadata,
      );
      _activeReasoningMessageIds.remove(event.subagentRunId);
      return true;
    }

    return null;
  }

  bool? _handleToolEvent(BaseEvent event) {
    if (event is ToolCallStartEvent) {
      _activeToolCallIds[event.subagentRunId] = event.toolCallId;
      _projectTool(
        event.toolCallId,
        event.subagentRunId,
        name: event.toolCallName,
        metadata: event.metadata,
        isStreaming: true,
      );
      return true;
    }
    if (event is ToolCallArgsEvent) {
      _projectTool(
        event.toolCallId,
        event.subagentRunId,
        argsDelta: event.delta,
        metadata: event.metadata,
      );
      return true;
    }
    if (event is ToolCallChunkEvent) {
      final toolCallId =
          event.toolCallId ??
          _activeToolCallIds[event.subagentRunId] ??
          uid('tool');
      _activeToolCallIds[event.subagentRunId] = toolCallId;
      _projectTool(
        toolCallId,
        event.subagentRunId,
        name: event.toolCallName,
        argsDelta: event.delta,
        metadata: event.metadata,
        isStreaming: true,
      );
      return true;
    }
    if (event is ToolCallEndEvent) {
      _projectTool(
        event.toolCallId,
        event.subagentRunId,
        metadata: event.metadata,
        isStreaming: false,
      );
      return true;
    }
    if (event is ToolCallResultEvent) {
      _projectTool(
        event.toolCallId,
        event.subagentRunId,
        result: event.content,
        metadata: event.metadata,
        isStreaming: false,
      );
      return true;
    }

    return null;
  }

  /// Reconciles an authoritative typed snapshot without using arrival order.
  void reconcileSnapshot(
    List<Message> snapshot, {
    bool projectToolCalls = true,
  }) {
    for (final message in snapshot) {
      final id = message.id;
      if (id == null || id.isEmpty) continue;
      final subagentRunId = message.subagentRunId;
      final name = _subagentName(subagentRunId);
      final status = _subagentStatus(subagentRunId);
      if (message is AssistantMessage) {
        _upsert(
          ChatMessage(
            id: _displayId(subagentRunId, id, ChatMessageType.assistant),
            type: ChatMessageType.assistant,
            content: message.content ?? '',
            timestamp: DateTime.now(),
            metadata: message.metadata,
            protocolId: id,
            subagentRunId: subagentRunId,
            subagentName: name,
            subagentStatus: status,
          ),
        );
      } else if (message is ReasoningMessage) {
        final content = message.content ?? message.thinking ?? '';
        if (content.isEmpty) continue;
        _upsert(
          ChatMessage(
            id: _displayId(subagentRunId, id, ChatMessageType.reasoning),
            type: ChatMessageType.reasoning,
            content: content,
            timestamp: DateTime.now(),
            metadata: message.metadata,
            protocolId: id,
            subagentRunId: subagentRunId,
            subagentName: name,
            subagentStatus: status,
          ),
        );
      } else if (message is ToolMessage) {
        _projectTool(
          message.toolCallId,
          subagentRunId,
          result: message.content,
          metadata: message.metadata,
          isStreaming: false,
        );
      } else if (message is ActivityMessage) {
        _upsert(
          ChatMessage(
            id: _displayId(subagentRunId, id, ChatMessageType.system),
            type: ChatMessageType.system,
            content: 'Activity: ${message.activityType}',
            timestamp: DateTime.now(),
            metadata: message.metadata,
            protocolId: id,
            subagentRunId: subagentRunId,
            subagentName: name,
            subagentStatus: status,
          ),
        );
      }
      if (projectToolCalls && message is AssistantMessage) {
        for (final toolCall in message.toolCalls ?? const <ToolCall>[]) {
          _projectTool(
            toolCall.id,
            subagentRunId,
            name: toolCall.function.name,
            args: toolCall.function.arguments,
            metadata: toolCall.metadata,
            isStreaming: false,
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
    _activeTextMessageIds.clear();
    _activeReasoningMessageIds.clear();
    _activeToolCallIds.clear();
  }

  void _startReasoning(
    String messageId,
    String? subagentRunId,
    DateTime timestamp,
    Metadata? metadata,
  ) {
    final displayId = _displayId(
      subagentRunId,
      messageId,
      ChatMessageType.reasoning,
    );
    _activeReasoningMessageIds[subagentRunId] = displayId;
    _upsert(
      ChatMessage(
        id: displayId,
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

  void _appendText(
    String protocolId,
    String? subagentRunId,
    String delta, {
    Metadata? metadata,
    bool create = false,
  }) {
    _appendDisplay(
      _displayId(subagentRunId, protocolId, ChatMessageType.assistant),
      ChatMessageType.assistant,
      delta,
      metadata: metadata,
      protocolId: protocolId,
      subagentRunId: subagentRunId,
      create: create,
    );
  }

  void _appendReasoning(
    String protocolId,
    String? subagentRunId,
    String delta, {
    Metadata? metadata,
    bool create = false,
  }) {
    _appendDisplay(
      _displayId(subagentRunId, protocolId, ChatMessageType.reasoning),
      ChatMessageType.reasoning,
      delta,
      protocolId: protocolId,
      metadata: metadata,
      subagentRunId: subagentRunId,
      create: create,
    );
  }

  void _appendDisplay(
    String displayId,
    ChatMessageType type,
    String delta, {
    String? protocolId,
    Metadata? metadata,
    String? subagentRunId,
    bool create = false,
  }) {
    final index = type == ChatMessageType.assistant
        ? _textMessageIndex(displayId)
        : _messageIndex(displayId, type);
    if (index < 0) {
      if (!create) return;
      _upsert(
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

  void _finishMessage(
    String protocolId,
    String? subagentRunId,
    ChatMessageType type, {
    Metadata? metadata,
  }) {
    final displayId = _displayId(subagentRunId, protocolId, type);
    final index = type == ChatMessageType.assistant
        ? _textMessageIndex(displayId)
        : _messageIndex(displayId, type);
    if (index < 0) return;
    final previous = messages[index];
    messages[index] = previous.copyWith(
      isStreaming: false,
      metadata: mergeMetadata(previous.metadata, metadata),
    );
  }

  void _projectTool(
    String protocolId,
    String? subagentRunId, {
    String? name,
    String? args,
    String? argsDelta,
    String? result,
    Metadata? metadata,
    bool? isStreaming,
  }) {
    final displayId = _toolDisplayId(subagentRunId, protocolId);
    final index = _messageIndex(displayId, ChatMessageType.tool);
    final previous = index < 0 ? null : messages[index];
    final toolName = name ?? previous?.toolName ?? 'Tool';
    final toolArgs = args ?? '${previous?.toolArgs ?? ''}${argsDelta ?? ''}';
    final toolResult = result ?? previous?.toolResult;
    _upsert(
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

  void _setSubagent(
    String id, {
    String? name,
    String? description,
    String? parentSubagentRunId,
    String? parentToolCallId,
    String? parentMessageId,
    AgUiSubagentStatus? status,
    dynamic result,
    List<String>? interruptIds,
    Metadata? metadata,
  }) {
    final previous = _subagents[id];
    final next = (previous ?? AgUiSubagentState(subagentRunId: id)).copyWith(
      name: name ?? _unsetProjectionField,
      description: description ?? _unsetProjectionField,
      parentSubagentRunId: parentSubagentRunId ?? _unsetProjectionField,
      parentToolCallId: parentToolCallId ?? _unsetProjectionField,
      parentMessageId: parentMessageId ?? _unsetProjectionField,
      status: status,
      result: result,
      interruptIds: interruptIds,
      metadata: mergeMetadata(previous?.metadata, metadata),
    );
    _subagents[id] = next;
    _refreshSubagentMessages(id, next);
  }

  void _refreshSubagentMessages(String id, AgUiSubagentState state) {
    for (var i = 0; i < messages.length; i++) {
      final message = messages[i];
      if (message.subagentRunId != id) continue;
      messages[i] = message.copyWith(
        metadata: mergeMetadata(message.metadata, state.metadata),
        subagentName: state.name,
        subagentStatus: state.status.label,
      );
    }
  }

  String? _subagentName(String? id) => id == null ? null : _subagents[id]?.name;

  String? _subagentStatus(String? id) =>
      id == null ? null : _subagents[id]?.status.label;

  ChatMessageType _textType(TextMessageRole? role) => switch (role) {
    TextMessageRole.user => ChatMessageType.user,
    TextMessageRole.system ||
    TextMessageRole.developer => ChatMessageType.system,
    _ => ChatMessageType.assistant,
  };

  String _displayId(
    String? subagentRunId,
    String protocolId,
    ChatMessageType type,
  ) {
    final key = (subagentRunId, protocolId, type);
    return _messageDisplayIds.putIfAbsent(
      key,
      () => subagentRunId == null ? protocolId : uid('subagent-message'),
    );
  }

  String _toolDisplayId(String? subagentRunId, String protocolId) {
    final key = (subagentRunId, protocolId);
    return _toolDisplayIds.putIfAbsent(
      key,
      () => subagentRunId == null ? protocolId : uid('subagent-tool'),
    );
  }

  void _upsert(ChatMessage message) {
    final index = _messageIndex(message.id, message.type);
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

  int _messageIndex(String displayId, ChatMessageType type) => messages
      .indexWhere((message) => message.id == displayId && message.type == type);

  int _textMessageIndex(String displayId) => messages.indexWhere(
    (message) =>
        message.id == displayId &&
        (message.type == ChatMessageType.assistant ||
            message.type == ChatMessageType.user ||
            message.type == ChatMessageType.system),
  );

  DateTime _timestamp(BaseEvent event) => event.timestamp == null
      ? DateTime.now()
      : DateTime.fromMillisecondsSinceEpoch(event.timestamp!);
}
