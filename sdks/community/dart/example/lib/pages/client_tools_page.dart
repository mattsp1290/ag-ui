import 'dart:async';
import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:ag_ui/ag_ui.dart';
import '../models/chat_message.dart';
import '../models/endpoint_config.dart';
import '../services/ag_ui_service.dart';
import '../services/ids.dart';
import '../widgets/chat_message_widget.dart';
import '../widgets/chat_input_widget.dart';
import '../widgets/card_widget.dart';
import '../widgets/approval_card_widget.dart';
import 'agui_event_handling.dart';

/// Page for the client-tool round-trip features:
///  - `agentic_chat` (FeatureKind.clientTools) — model calls a tool, client executes,
///    result returns inline.
///  - `tool_based_generative_ui` (FeatureKind.clientTools) — model calls `render_card`,
///    the call is rendered AS a UI card.
///  - `human_in_the_loop` (FeatureKind.approval) — the tool call is gated by an
///    approve/deny decision; the decision becomes the tool result.
class ClientToolsPage extends StatelessWidget {
  final EndpointConfig endpoint;

  const ClientToolsPage({super.key, required this.endpoint});

  @override
  Widget build(BuildContext context) {
    return ChangeNotifierProvider<ClientToolsPageState>(
      create: (_) => ClientToolsPageState(endpoint: endpoint),
      child: const ClientToolsPageView(),
    );
  }
}

class ClientToolsPageView extends StatelessWidget {
  const ClientToolsPageView({super.key});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final state = context.watch<ClientToolsPageState>();

    return Scaffold(
      backgroundColor: theme.colorScheme.surface,
      appBar: AppBar(
        title: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(state.endpoint.name, style: theme.textTheme.titleMedium),
            Text(
              state.endpoint.description,
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
          ],
        ),
        actions: [
          if (state.isApproval)
            Row(
              children: [
                const Text('Gate', style: TextStyle(fontSize: 12)),
                Switch(
                  value: state.approvalGate,
                  onChanged: state.busy ? null : state.setApprovalGate,
                ),
              ],
            ),
        ],
        bottom: state.busy
            ? const PreferredSize(
                preferredSize: Size.fromHeight(2),
                child: LinearProgressIndicator(),
              )
            : null,
      ),
      body: Column(
        children: [
          Expanded(
            child: state.messages.isEmpty
                ? _EmptyState(endpoint: state.endpoint)
                : ListView.builder(
                    reverse: true,
                    padding: const EdgeInsets.symmetric(vertical: 16),
                    itemCount: state.messages.length,
                    itemBuilder: (context, index) {
                      final msg =
                          state.messages[state.messages.length - 1 - index];
                      if (msg.type == ChatMessageType.card &&
                          msg.cardData != null) {
                        return CardWidget(data: msg.cardData!);
                      }
                      return ChatMessageWidget(message: msg);
                    },
                  ),
          ),
          if (state.pendingApproval != null)
            ApprovalCardWidget(
              summary: state.pendingApproval!,
              onApprove: state.approve,
              onDeny: state.deny,
            ),
          ChatInputWidget(
            onSendMessage: state.sendMessage,
            isEnabled: !state.busy,
          ),
        ],
      ),
    );
  }
}

class _EmptyState extends StatelessWidget {
  final EndpointConfig endpoint;
  const _EmptyState({required this.endpoint});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(endpoint.icon, size: 64, color: theme.colorScheme.outline),
          const SizedBox(height: 16),
          Text(
            endpoint.featureKind == FeatureKind.approval
                ? 'Ask the agent to do something consequential'
                : 'Ask the agent something it can use a tool for',
            style: theme.textTheme.titleMedium?.copyWith(
              color: theme.colorScheme.outline,
            ),
          ),
        ],
      ),
    );
  }
}

class ClientToolsPageState extends ChangeNotifier with AgUiEventHandling {
  final EndpointConfig endpoint;
  final AgUiService _service;
  final String _threadId = uid('thread');
  final List<Message> _history = [];
  final Set<(String?, String)> _settledCalls = {};
  List<({ToolCall call, String? subagentRunId})> _pendingCalls = const [];
  bool _busy = false;
  bool _aborted = false;

  static const maxFollowUpRuns = 8;

  bool _approvalGate = true;
  String? _pendingApproval;
  Completer<bool>? _approvalCompleter;

  ClientToolsPageState({required this.endpoint, AgUiService? service})
    : _service = service ?? AgUiService();

  bool get busy => _busy;
  bool get isApproval => endpoint.featureKind == FeatureKind.approval;
  bool get approvalGate => _approvalGate;
  String? get pendingApproval => _pendingApproval;

  void _notify() {
    if (!disposed) notifyListeners();
  }

  void setApprovalGate(bool value) {
    if (disposed || _busy) return;
    _approvalGate = value;
    _notify();
  }

  @override
  void onRunReset() {
    _aborted = true;
    _settleAbandonedProposals();
    _pendingCalls = const [];
  }

  /// One owner awaits the entire exchange, including local approval decisions.
  Future<void> sendMessage(String text) async {
    if (disposed || text.trim().isEmpty || _busy) return;
    _pendingCalls = const [];
    _aborted = false;
    _busy = true;
    final id = uid('user');
    _history.add(UserMessage(id: id, content: text.trim()));
    messages.add(
      ChatMessage(
        id: id,
        type: ChatMessageType.user,
        content: text.trim(),
        timestamp: DateTime.now(),
      ),
    );
    _notify();

    try {
      for (var followUps = 0; ; followUps++) {
        if (!await _consumeRun(continuation: followUps > 0)) return;
        if (_pendingCalls.isEmpty) return;
        if (followUps == maxFollowUpRuns) {
          throw StateError(
            'Stopped after $maxFollowUpRuns tool follow-up runs. Submit another message to continue.',
          );
        }
        final calls = _pendingCalls;
        _pendingCalls = const [];
        for (final pending in calls) {
          final call = pending.call;
          final identity = (pending.subagentRunId, call.id);
          if (!_settledCalls.add(identity)) continue;
          final result = await _execute(
            call,
            subagentRunId: pending.subagentRunId,
          );
          if (disposed || _aborted) return;
          _history.add(
            ToolMessage(
              id: uid('tool'),
              toolCallId: call.id,
              content: result,
              subagentRunId: pending.subagentRunId,
            ),
          );
        }
        if (disposed || _aborted) return;
      }
    } catch (error) {
      if (!disposed) _addError(error);
    } finally {
      _pendingCalls = const [];
      if (!disposed) finishStreaming();
      _busy = false;
      _notify();
    }
  }

  Future<bool> _consumeRun({required bool continuation}) async {
    final priorMessageIds = messages.map((message) => message.id).toSet();
    beginRun(continuation: continuation);
    _pendingCalls = const [];
    await for (final event in _service.run(
      endpoint.path,
      threadId: _threadId,
      messages: _history,
      tools: endpoint.tools,
      extraQuery: isApproval && !_approvalGate ? {'approval': 'off'} : const {},
    )) {
      if (disposed || _aborted) return false;
      if (event is MessagesSnapshotEvent) {
        _mergeHistory(event.messages);
        reconcileSnapshot(event.messages, projectToolCalls: false);
        _pendingCalls = [
          for (final assistant in event.messages.whereType<AssistantMessage>())
            for (final call in assistant.toolCalls ?? const <ToolCall>[])
              if (!_settledCalls.contains((assistant.subagentRunId, call.id)))
                (call: call, subagentRunId: assistant.subagentRunId),
        ];
      } else {
        handleCommonEvent(event);
      }
      _notify();
      if (event is RunErrorEvent || _aborted) return false;
      if (event is RunFinishedEvent) {
        // Production sends a final snapshot; preserve streamed replies too when
        // a peer finishes a valid text-only run without a snapshot.
        for (final message in messages) {
          if (priorMessageIds.contains(message.id)) continue;
          final protocolId = message.protocolId ?? message.id;
          if (_history.any(
            (existing) =>
                existing.id == protocolId &&
                existing.subagentRunId == message.subagentRunId,
          )) {
            continue;
          }
          if (message.type == ChatMessageType.assistant) {
            _history.add(
              AssistantMessage(
                id: protocolId,
                content: message.content,
                metadata: message.metadata,
                subagentRunId: message.subagentRunId,
              ),
            );
          } else if (message.type == ChatMessageType.reasoning) {
            _history.add(
              ReasoningMessage(
                id: protocolId,
                content: message.content,
                metadata: message.metadata,
                subagentRunId: message.subagentRunId,
              ),
            );
          }
        }
        finishStreaming();
        return true;
      }
    }
    if (!disposed && !_aborted) {
      throw StateError('The run was interrupted before it completed.');
    }
    return false;
  }

  void _mergeHistory(List<Message> snapshot) {
    for (final message in snapshot) {
      final id = message.id;
      if (id == null || id.isEmpty) {
        throw const FormatException(
          'Snapshot message is missing its protocol ID.',
        );
      }
      final index = _history.indexWhere(
        (previous) =>
            previous.id == id &&
            previous.subagentRunId == message.subagentRunId,
      );
      if (index < 0) {
        _history.add(message);
      } else {
        _history[index] = message;
      }
      if (message is ToolMessage) {
        _settledCalls.add((message.subagentRunId, message.toolCallId));
      }
    }
  }

  /// Execute one tool call and return the JSON result string the model will read.
  Future<String> _execute(ToolCall call, {String? subagentRunId}) async {
    try {
      final name = call.function.name;
      if (!endpoint.tools.any((tool) => tool.name == name)) {
        throw FormatException('Unknown tool: $name');
      }
      final args = _parseArgs(call.function.arguments);
      switch (name) {
        case 'get_current_time':
          final result = jsonEncode({'time': DateTime.now().toIso8601String()});
          _addToolBubble(call, args, result, subagentRunId: subagentRunId);
          return result;
        case 'calculate':
          final expression = _requiredString(args, 'expression');
          final result = _calculate(expression);
          _addToolBubble(call, args, result, subagentRunId: subagentRunId);
          return result;
        case 'render_card':
          final title = _requiredString(args, 'title');
          final facts = args['facts'];
          if (facts != null &&
              (facts is! List ||
                  facts.any(
                    (fact) =>
                        fact is! Map<String, dynamic> ||
                        fact['label'] is! String ||
                        fact['value'] is! String,
                  ))) {
            throw const FormatException(
              'facts must contain label/value strings',
            );
          }
          for (final key in ['subtitle', 'imageUrl']) {
            if (args[key] != null && args[key] is! String) {
              throw FormatException('$key must be a string');
            }
          }
          messages.add(
            ChatMessage(
              id: uid('card'),
              type: ChatMessageType.card,
              content: title,
              timestamp: DateTime.now(),
              cardData: args,
              metadata: call.metadata,
              protocolId: call.id,
              subagentRunId: subagentRunId,
              subagentName: subagents[subagentRunId]?.name,
              subagentStatus: subagents[subagentRunId]?.status.label,
            ),
          );
          _notify();
          return jsonEncode({'rendered': true});
        case 'request_approval':
          final summary = _requiredString(args, 'summary');
          final action = _requiredString(args, 'action');
          if (isApproval && _approvalGate) {
            return await _requestApproval(summary, action);
          }
          return jsonEncode({
            'approved': true,
            'result': 'Local demonstration only; approval gate is off.',
          });
        default:
          throw FormatException('Unknown tool: $name');
      }
    } catch (error) {
      return jsonEncode({
        'error': {'code': 'invalid_tool_call', 'message': error.toString()},
      });
    }
  }

  String _requiredString(Map<String, dynamic> args, String key) {
    final value = args[key];
    if (value is! String || value.trim().isEmpty) {
      throw FormatException('$key must be a nonempty string');
    }
    return value;
  }

  Future<String> _requestApproval(String summary, String action) async {
    // Guard the whole flow against disposal so a multi-approval batch can't hang or
    // notify after dispose. dispose() completes any in-flight completer with false.
    if (disposed) return jsonEncode({'approved': false, 'reason': 'cancelled'});
    _pendingApproval = '$summary\n\nAction: $action';
    _approvalCompleter = Completer<bool>();
    _notify();

    final approved = await _approvalCompleter!.future;
    _pendingApproval = null;
    _approvalCompleter = null;
    if (disposed) return jsonEncode({'approved': false, 'reason': 'cancelled'});

    messages.add(
      ChatMessage(
        id: uid('decision'),
        type: ChatMessageType.system,
        content: approved
            ? '✅ Approved: request_approval'
            : '🚫 Denied: request_approval',
        timestamp: DateTime.now(),
      ),
    );
    _notify();

    if (!approved) {
      return jsonEncode({
        'approved': false,
        'reason': 'The user declined this action.',
      });
    }
    return jsonEncode({
      'approved': true,
      'result': 'The user approved. You may proceed with: $action.',
    });
  }

  /// Renders an agentic_chat tool call + its result as a visible bubble, so the
  /// invocation is shown (plan 02), not just the model's final text.
  void _addToolBubble(
    ToolCall call,
    Map<String, dynamic> args,
    String result, {
    String? subagentRunId,
  }) {
    final name = call.function.name;
    final subagent = subagents[subagentRunId];
    messages.add(
      ChatMessage(
        id: uid('tool'),
        type: ChatMessageType.tool,
        content: args.isEmpty
            ? '$name()\n→ $result'
            : '$name(${jsonEncode(args)})\n→ $result',
        timestamp: DateTime.now(),
        toolName: name,
        toolArgs: args,
        toolResult: result,
        metadata: call.metadata,
        protocolId: call.id,
        subagentRunId: subagentRunId,
        subagentName: subagent?.name,
        subagentStatus: subagent?.status.label,
      ),
    );
    _notify();
  }

  // approve()/deny() are only valid while [pendingApproval] != null — the approval
  // panel (the only caller) is shown exactly then, so UI gating is the guard. Both
  // no-op safely if no completer is pending.
  void approve() => _resolveDecision(true);
  void deny() => _resolveDecision(false);

  void _resolveDecision(bool v) {
    final c = _approvalCompleter;
    if (c != null && !c.isCompleted) c.complete(v);
  }

  /// Tiny arithmetic evaluator for the `calculate` demo tool. Returns a JSON result;
  /// never throws (errors become `{"error": ...}` so the model can recover).
  String _calculate(String expr) {
    try {
      final value = _evalExpression(expr);
      return jsonEncode({'result': value});
    } catch (e) {
      return jsonEncode({'error': 'could not evaluate "$expr"'});
    }
  }

  Map<String, dynamic> _parseArgs(String raw) {
    if (raw.trim().isEmpty) return <String, dynamic>{};
    final decoded = jsonDecode(raw);
    if (decoded is! Map<String, dynamic>) {
      throw const FormatException('Tool arguments must be an object');
    }
    return decoded;
  }

  // A failed/incomplete run must not leave dangling proposals in the next
  // request's provider history. Record cancellation as data without executing.
  void _settleAbandonedProposals() {
    final results = _history
        .whereType<ToolMessage>()
        .map((message) => (message.subagentRunId, message.toolCallId))
        .toSet();
    final calls = [
      for (final message in _history.whereType<AssistantMessage>())
        for (final call in message.toolCalls ?? const <ToolCall>[])
          (call: call, subagentRunId: message.subagentRunId),
    ];
    for (final pending in calls) {
      final identity = (pending.subagentRunId, pending.call.id);
      if (!results.add(identity)) continue;
      _settledCalls.add(identity);
      _history.add(
        ToolMessage(
          id: uid('tool'),
          toolCallId: pending.call.id,
          subagentRunId: pending.subagentRunId,
          content: jsonEncode({
            'error': {
              'code': 'exchange_stopped',
              'message':
                  'The local exchange ended before this tool could complete.',
            },
          }),
        ),
      );
    }
  }

  void _addError(Object e) {
    _settleAbandonedProposals();
    messages.add(
      ChatMessage(
        id: uid('error'),
        type: ChatMessageType.system,
        content: 'Error: $e',
        timestamp: DateTime.now(),
      ),
    );
    _notify();
  }

  @override
  void dispose() {
    if (disposed) return;
    disposed = true;
    // Unwind the exchange without running a continuation after navigation.
    if (_approvalCompleter != null && !_approvalCompleter!.isCompleted) {
      _approvalCompleter!.complete(false);
    }
    _service.dispose();
    super.dispose();
  }
}

/// Shunting-yard evaluator for +, -, *, /, parentheses over doubles.
num _evalExpression(String input) {
  final tokens = _tokenize(input);
  final output = <Object>[]; // numbers (num) and operators (String)
  final ops = <String>[];
  const prec = {'+': 1, '-': 1, '*': 2, '/': 2};

  for (final t in tokens) {
    if (t is num) {
      output.add(t);
    } else if (t == '(') {
      ops.add(t as String);
    } else if (t == ')') {
      while (ops.isNotEmpty && ops.last != '(') {
        output.add(ops.removeLast());
      }
      if (ops.isEmpty) throw const FormatException('mismatched parens');
      ops.removeLast();
    } else {
      while (ops.isNotEmpty &&
          ops.last != '(' &&
          prec[ops.last]! >= prec[t as String]!) {
        output.add(ops.removeLast());
      }
      ops.add(t as String);
    }
  }
  while (ops.isNotEmpty) {
    final op = ops.removeLast();
    if (op == '(') throw const FormatException('mismatched parens');
    output.add(op);
  }

  final stack = <num>[];
  for (final tok in output) {
    if (tok is num) {
      stack.add(tok);
    } else {
      if (stack.length < 2) throw const FormatException('bad expression');
      final b = stack.removeLast();
      final a = stack.removeLast();
      switch (tok as String) {
        case '+':
          stack.add(a + b);
        case '-':
          stack.add(a - b);
        case '*':
          stack.add(a * b);
        case '/':
          if (b == 0) throw const FormatException('divide by zero');
          stack.add(a / b);
      }
    }
  }
  if (stack.length != 1) throw const FormatException('bad expression');
  return stack.single;
}

final _numChar = RegExp(r'[0-9.]');

List<Object> _tokenize(String input) {
  final tokens = <Object>[];
  final s = input.replaceAll(' ', '');
  int i = 0;
  // True where a value is expected: at the start, after an operator, or after '('.
  // A '-' in that position is a unary minus on the following numeric literal.
  bool expectValue = true;
  while (i < s.length) {
    final c = s[i];
    if (c == '-' &&
        expectValue &&
        i + 1 < s.length &&
        _numChar.hasMatch(s[i + 1])) {
      final start = i;
      i++; // consume '-'
      while (i < s.length && _numChar.hasMatch(s[i])) {
        i++;
      }
      tokens.add(num.parse(s.substring(start, i))); // negative literal
      expectValue = false;
    } else if ('+-*/()'.contains(c)) {
      tokens.add(c);
      i++;
      expectValue =
          c != ')'; // after ')' a value is not expected; after op/'(' it is
    } else {
      final start = i;
      while (i < s.length && _numChar.hasMatch(s[i])) {
        i++;
      }
      if (i == start) throw FormatException('unexpected char "$c"');
      tokens.add(num.parse(s.substring(start, i)));
      expectValue = false;
    }
  }
  return tokens;
}
