import 'package:ag_ui/ag_ui.dart';
import 'package:flutter/foundation.dart';

import '../models/agui_event_projection.dart';
import '../models/chat_message.dart';

export '../models/agui_event_projection.dart'
    show AgUiRunStatus, AgUiSubagentState, AgUiSubagentStatus;

mixin AgUiEventHandling on ChangeNotifier {
  bool disposed = false;
  late final AgUiEventProjection _projection = AgUiEventProjection(
    onRunError: onRunReset,
  );

  List<ChatMessage> get messages => _projection.messages;
  bool get runIsTerminal => _projection.runIsTerminal;
  AgUiRunStatus get runStatus => _projection.runStatus;
  bool get runIsAwaitingInput => _projection.runIsAwaitingInput;
  RunFinishedOutcome? get runOutcome => _projection.runOutcome;
  List<TokenUsage> get runUsage => _projection.runUsage;
  Map<String, AgUiSubagentState> get subagents => _projection.subagents;

  void onRunReset() {}

  void beginRun({Iterable<String> resumedSubagentIds = const []}) =>
      _projection.beginRun(resumedSubagentIds: resumedSubagentIds);

  bool handleCommonEvent(BaseEvent event) => _projection.handleEvent(event);

  void reconcileSnapshot(
    List<Message> snapshot, {
    bool projectToolCalls = true,
  }) => _projection.reconcileSnapshot(
    snapshot,
    projectToolCalls: projectToolCalls,
  );

  void addReasoningMessage(String text, {String? id}) =>
      _projection.addReasoningMessage(text, id: id);

  void finishStreaming() => _projection.finishStreaming();
}
