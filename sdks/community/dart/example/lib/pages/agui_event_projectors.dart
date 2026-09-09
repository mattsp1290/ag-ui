part of 'agui_event_projection.dart';

extension _AgUiEventProjectors on AgUiEventProjection {
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
      final displayId = _store.displayId(
        event.subagentRunId,
        event.messageId,
        ChatMessageType.assistant,
      );
      _store.activeTextMessageIds[event.subagentRunId] = displayId;
      _store.upsert(
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
      _store.appendText(
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
          ? (_store.activeTextMessageIds[event.subagentRunId] ??
                _store.displayId(
                  event.subagentRunId,
                  uid('assistant'),
                  ChatMessageType.assistant,
                ))
          : _store.displayId(
              event.subagentRunId,
              event.messageId!,
              ChatMessageType.assistant,
            );
      _store.activeTextMessageIds[event.subagentRunId] = displayId;
      _store.appendDisplay(
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
      _store.finishMessage(
        event.messageId,
        event.subagentRunId,
        ChatMessageType.assistant,
        metadata: event.metadata,
      );
      final displayId = _store.displayId(
        event.subagentRunId,
        event.messageId,
        ChatMessageType.assistant,
      );
      if (_store.activeTextMessageIds[event.subagentRunId] == displayId) {
        _store.activeTextMessageIds.remove(event.subagentRunId);
      }
      return true;
    }

    return null;
  }

  bool? _handleReasoningEvent(BaseEvent event) {
    if (event is ReasoningMessageStartEvent) {
      _store.startReasoning(
        event.messageId,
        event.subagentRunId,
        _timestamp(event),
        event.metadata,
      );
      return true;
    }
    if (event is ReasoningStartEvent) {
      _store.startReasoning(
        event.messageId,
        event.subagentRunId,
        _timestamp(event),
        event.metadata,
      );
      return true;
    }
    if (event is ReasoningMessageContentEvent) {
      _store.appendReasoning(
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
          ? (_store.activeReasoningMessageIds[event.subagentRunId] ??
                _store.displayId(
                  event.subagentRunId,
                  uid('reasoning'),
                  ChatMessageType.reasoning,
                ))
          : _store.displayId(
              event.subagentRunId,
              event.messageId!,
              ChatMessageType.reasoning,
            );
      _store.activeReasoningMessageIds[event.subagentRunId] = displayId;
      _store.appendDisplay(
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
      _store.finishMessage(
        event.messageId,
        event.subagentRunId,
        ChatMessageType.reasoning,
        metadata: event.metadata,
      );
      return true;
    }
    if (event is ReasoningEndEvent) {
      _store.finishMessage(
        event.messageId,
        event.subagentRunId,
        ChatMessageType.reasoning,
        metadata: event.metadata,
      );
      _store.activeReasoningMessageIds.remove(event.subagentRunId);
      return true;
    }

    return null;
  }

  bool? _handleToolEvent(BaseEvent event) {
    if (event is ToolCallStartEvent) {
      _store.activeToolCallIds[event.subagentRunId] = event.toolCallId;
      _store.projectTool(
        event.toolCallId,
        event.subagentRunId,
        name: event.toolCallName,
        metadata: event.metadata,
        isStreaming: true,
      );
      return true;
    }
    if (event is ToolCallArgsEvent) {
      _store.projectTool(
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
          _store.activeToolCallIds[event.subagentRunId] ??
          uid('tool');
      _store.activeToolCallIds[event.subagentRunId] = toolCallId;
      _store.projectTool(
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
      _store.projectTool(
        event.toolCallId,
        event.subagentRunId,
        metadata: event.metadata,
        isStreaming: false,
      );
      return true;
    }
    if (event is ToolCallResultEvent) {
      _store.projectTool(
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
  void _reconcileSnapshot(
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
        _store.upsert(
          ChatMessage(
            id: _store.displayId(subagentRunId, id, ChatMessageType.assistant),
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
        _store.upsert(
          ChatMessage(
            id: _store.displayId(subagentRunId, id, ChatMessageType.reasoning),
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
        _store.projectTool(
          message.toolCallId,
          subagentRunId,
          result: message.content,
          metadata: message.metadata,
          isStreaming: false,
        );
      } else if (message is ActivityMessage) {
        _store.upsert(
          ChatMessage(
            id: _store.displayId(subagentRunId, id, ChatMessageType.system),
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
          _store.projectTool(
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
}
