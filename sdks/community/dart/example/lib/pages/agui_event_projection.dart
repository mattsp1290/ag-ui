import 'package:ag_ui/ag_ui.dart';
import '../models/chat_message.dart';
import '../services/ids.dart';

part 'agui_event_projectors.dart';
part 'agui_projection_store.dart';

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
  final Map<String, AgUiSubagentState> _subagents = {};
  late final _AgUiProjectionStore _store = _AgUiProjectionStore(
    subagentName: _subagentName,
    subagentStatus: _subagentStatus,
  );
  List<TokenUsage> _runUsage = const [];
  RunFinishedOutcome? _runOutcome;
  AgUiRunStatus _runStatus = AgUiRunStatus.idle;

  List<ChatMessage> get messages => _store.messages;
  bool get runIsTerminal => _terminal;
  AgUiRunStatus get runStatus => _runStatus;
  bool get runIsAwaitingInput => _runStatus == AgUiRunStatus.awaitingInput;
  RunFinishedOutcome? get runOutcome => _runOutcome;
  List<TokenUsage> get runUsage => List.unmodifiable(_runUsage);
  Map<String, AgUiSubagentState> get subagents => Map.unmodifiable(_subagents);

  final void Function()? onRunError;

  AgUiEventProjection({this.onRunError});

  void beginRun({
    Iterable<String> resumedSubagentIds = const [],
    bool continuation = false,
  }) {
    final retainedSubagentIds = continuation
        ? _subagents.keys.toList()
        : resumedSubagentIds;
    final resumed = <String, AgUiSubagentState>{
      for (final id in retainedSubagentIds)
        if (_subagents[id] case final state?)
          id: state.copyWith(
            status: AgUiSubagentStatus.running,
            result: null,
            interruptIds: null,
          ),
    };
    _store.beginRun(retainedSubagentIds, preserveAll: continuation);
    _terminal = false;
    _runStatus = AgUiRunStatus.running;
    _runOutcome = null;
    _runUsage = const [];
    _subagents.clear();
    _subagents.addAll(resumed);
    for (final entry in resumed.entries) {
      _store.refreshSubagentMessages(entry.key, entry.value);
    }
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

  void addReasoningMessage(String text, {String? id}) =>
      _store.addReasoningMessage(text, id: id);

  void reconcileSnapshot(
    List<Message> snapshot, {
    bool projectToolCalls = true,
  }) => _reconcileSnapshot(snapshot, projectToolCalls: projectToolCalls);

  void finishStreaming() => _store.finishStreaming();

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
    _store.refreshSubagentMessages(id, next);
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

  DateTime _timestamp(BaseEvent event) => event.timestamp == null
      ? DateTime.now()
      : DateTime.fromMillisecondsSinceEpoch(event.timestamp!);
}
