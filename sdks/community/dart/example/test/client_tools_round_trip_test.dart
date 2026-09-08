import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:ag_ui/ag_ui.dart';
import 'package:ag_ui_example/models/chat_message.dart';
import 'package:ag_ui_example/models/endpoint_config.dart';
import 'package:ag_ui_example/services/ag_ui_service.dart';
import 'package:ag_ui_example/pages/client_tools_page.dart';

/// A fake service that yields a scripted list of events per `run()` call, so the
/// round-trip loop can be driven deterministically without a server.
class FakeAgUiService extends AgUiService {
  final List<List<BaseEvent>> runs;
  int calls = 0;
  final histories = <List<Message>>[];
  final threads = <String>[];
  final definitions = <List<Tool>>[];
  final queries = <Map<String, String>>[];
  FakeAgUiService(this.runs);

  @override
  Stream<BaseEvent> run(
    String endpoint, {
    required String threadId,
    required List<Message> messages,
    List<Tool> tools = const [],
    dynamic state,
    Map<String, String> extraQuery = const {},
  }) async* {
    histories.add(messages.map((m) => Message.fromJson(m.toJson())).toList());
    threads.add(threadId);
    definitions.add(List.of(tools));
    queries.add(Map.of(extraQuery));
    final events = calls < runs.length ? runs[calls] : const <BaseEvent>[];
    calls++;
    for (final e in events) {
      yield e;
    }
  }
}

EndpointConfig _clientToolsEndpoint() => const EndpointConfig(
  name: 'Agentic Chat',
  path: 'agentic_chat',
  description: 'test',
  icon: Icons.chat,
  featureKind: FeatureKind.clientTools,
  tools: [
    Tool(
      name: 'calculate',
      description: 'calc',
      parameters: {
        'type': 'object',
        'properties': {
          'expression': {'type': 'string'},
        },
      },
    ),
  ],
);

MessagesSnapshotEvent _snapshotWithToolCall(String callId, String expr) =>
    MessagesSnapshotEvent(
      messages: [
        AssistantMessage(
          id: 'a_$callId',
          toolCalls: [
            ToolCall(
              id: callId,
              function: FunctionCall(
                name: 'calculate',
                arguments: '{"expression":"$expr"}',
              ),
            ),
          ],
        ),
      ],
    );

final _runFinished = RunFinishedEvent(threadId: 't', runId: 'r');

int _countType(List<ChatMessage> ms, ChatMessageType t) =>
    ms.where((m) => m.type == t).length;

void main() {
  test(
    'happy round-trip: tool call → result → final answer; busy clears',
    () async {
      final service = FakeAgUiService([
        // Run A: model calls calculate, then finishes.
        [_snapshotWithToolCall('c1', '6*7'), _runFinished],
        // Run B: model answers in prose, then finishes.
        [
          TextMessageStartEvent(messageId: 'm1'),
          TextMessageContentEvent(messageId: 'm1', delta: '6*7 = 42'),
          TextMessageEndEvent(messageId: 'm1'),
          _runFinished,
        ],
      ]);
      final state = ClientToolsPageState(
        endpoint: _clientToolsEndpoint(),
        service: service,
      );

      state.sendMessage('what is 6*7?');
      await pumpEventQueue();

      expect(
        service.calls,
        2,
        reason: 'should re-run once after the tool result',
      );
      expect(state.busy, isFalse, reason: 'exchange converged');
      // A tool bubble was rendered (agentic_chat visibility) and the final text shown.
      expect(_countType(state.messages, ChatMessageType.tool), 1);
      final assistantTexts = state.messages
          .where((m) => m.type == ChatMessageType.assistant)
          .map((m) => m.content)
          .join();
      expect(assistantTexts, contains('42'));
      final toolBubble = state.messages.firstWhere(
        (m) => m.type == ChatMessageType.tool,
      );
      expect(
        toolBubble.content,
        contains('42'),
        reason: 'calculate(6*7) result is 42',
      );
    },
  );

  test('reasoning is deduped across cumulative snapshots', () async {
    final reasoning = ReasoningMessage(id: 'r1', content: 'let me think');
    final service = FakeAgUiService([
      // Run A: snapshot carries reasoning + a tool call.
      [
        MessagesSnapshotEvent(
          messages: [
            reasoning,
            AssistantMessage(
              id: 'a1',
              toolCalls: [
                ToolCall(
                  id: 'c1',
                  function: FunctionCall(
                    name: 'calculate',
                    arguments: '{"expression":"1+1"}',
                  ),
                ),
              ],
            ),
          ],
        ),
        _runFinished,
      ],
      // Run B: cumulative snapshot re-includes the SAME reasoning id + final answer.
      [
        MessagesSnapshotEvent(
          messages: [
            reasoning,
            AssistantMessage(id: 'a2', content: 'done'),
          ],
        ),
        _runFinished,
      ],
    ]);
    final state = ClientToolsPageState(
      endpoint: _clientToolsEndpoint(),
      service: service,
    );

    state.sendMessage('compute 1+1');
    await pumpEventQueue();

    expect(
      _countType(state.messages, ChatMessageType.reasoning),
      1,
      reason: 'same reasoning id must not be re-appended each round',
    );
  });

  test('RUN_ERROR mid-resolve does not launch a second resolve loop', () async {
    final service = FakeAgUiService([
      // Run A: a tool call → finish (launches the resolve loop).
      [_snapshotWithToolCall('c1', '2+2'), _runFinished],
      // Run B: errors, THEN the server (hypothetically) sends another tool call +
      // finish on the same stream. The loop must abort and ignore the trailing events.
      [
        RunErrorEvent(message: 'boom'),
        _snapshotWithToolCall('c2', '9+9'),
        _runFinished,
      ],
    ]);
    final state = ClientToolsPageState(
      endpoint: _clientToolsEndpoint(),
      service: service,
    );

    state.sendMessage('go');
    await pumpEventQueue();

    expect(state.busy, isFalse, reason: 'aborted run must clear busy');
    // Only c1 executed; c2 (post-error) must NOT have produced a second tool bubble.
    expect(
      _countType(state.messages, ChatMessageType.tool),
      1,
      reason: 'post-error tool call must not be resolved',
    );
    expect(
      state.messages.any(
        (m) =>
            m.type == ChatMessageType.system && m.content.contains('Run error'),
      ),
      isTrue,
    );
  });

  test('dispose during a pending approval unwinds without throwing', () async {
    final approvalEndpoint = const EndpointConfig(
      name: 'HITL',
      path: 'human_in_the_loop',
      description: 'test',
      icon: Icons.person,
      featureKind: FeatureKind.approval,
      tools: [
        Tool(
          name: 'request_approval',
          description: 'approve',
          parameters: {
            'type': 'object',
            'properties': {
              'summary': {'type': 'string'},
              'action': {'type': 'string'},
            },
          },
        ),
      ],
    );
    final service = FakeAgUiService([
      [
        MessagesSnapshotEvent(
          messages: [
            AssistantMessage(
              id: 'a1',
              toolCalls: [
                ToolCall(
                  id: 'c1',
                  function: FunctionCall(
                    name: 'request_approval',
                    arguments: '{"summary":"Delete it","action":"delete_file"}',
                  ),
                ),
              ],
            ),
          ],
        ),
        _runFinished,
      ],
    ]);
    final state = ClientToolsPageState(
      endpoint: approvalEndpoint,
      service: service,
    );

    state.sendMessage('delete the file');
    await pumpEventQueue();

    // The resolve loop is now awaiting the approval decision.
    expect(state.pendingApproval, isNotNull);

    // Disposing must complete the pending completer (false) and not throw.
    expect(() => state.dispose(), returnsNormally);
    await pumpEventQueue();
  });
  test(
    'history includes final replies and does not replay cumulative proposals',
    () async {
      final proposal = _snapshotWithToolCall('c1', '3*7');
      final service = FakeAgUiService([
        [proposal, proposal, _runFinished],
        [
          MessagesSnapshotEvent(
            messages: [
              ...proposal.messages,
              AssistantMessage(id: 'final-1', content: 'The answer is 21'),
            ],
          ),
          _runFinished,
        ],
        [
          MessagesSnapshotEvent(
            messages: [
              AssistantMessage(id: 'final-2', content: 'You are welcome'),
            ],
          ),
          _runFinished,
        ],
      ]);
      final state = ClientToolsPageState(
        endpoint: _clientToolsEndpoint(),
        service: service,
      );
      addTearDown(state.dispose);
      await state.sendMessage('calculate');
      await state.sendMessage('thanks');
      expect(service.calls, 3);
      final history = service.histories[2];
      expect(
        history.whereType<AssistantMessage>().where((m) => m.id == 'a_c1'),
        hasLength(1),
      );
      expect(
        history.whereType<ToolMessage>().where((m) => m.toolCallId == 'c1'),
        hasLength(1),
      );
      expect(
        history.whereType<AssistantMessage>().any(
          (m) => m.id == 'final-1' && m.content == 'The answer is 21',
        ),
        isTrue,
      );
      expect(service.threads.toSet(), hasLength(1));
      expect(
        service.definitions.every((tools) => tools.single.name == 'calculate'),
        isTrue,
      );
      expect(_countType(state.messages, ChatMessageType.tool), 1);
    },
  );

  for (final arguments in ['{broken', '[]', '{"expression":42}']) {
    test(
      'invalid tool arguments become a matching structured result: $arguments',
      () async {
        final service = FakeAgUiService([
          [
            MessagesSnapshotEvent(
              messages: [
                AssistantMessage(
                  id: 'proposal',
                  toolCalls: [
                    ToolCall(
                      id: 'bad-call',
                      function: FunctionCall(
                        name: 'calculate',
                        arguments: arguments,
                      ),
                    ),
                  ],
                ),
              ],
            ),
            _runFinished,
          ],
          [_runFinished],
        ]);
        final state = ClientToolsPageState(
          endpoint: _clientToolsEndpoint(),
          service: service,
        );
        addTearDown(state.dispose);
        await state.sendMessage('calculate');
        expect(service.calls, 2);
        final result = service.histories[1].whereType<ToolMessage>().single;
        expect(result.toolCallId, 'bad-call');
        expect(
          (jsonDecode(result.content) as Map)['error']['code'],
          'invalid_tool_call',
        );
        expect(state.busy, isFalse);
      },
    );
  }

  test(
    'eight follow-up budget stops an endless proposal sequence and allows recovery',
    () async {
      final service = FakeAgUiService([
        for (var i = 0; i < 9; i++)
          [_snapshotWithToolCall('c$i', '1+1'), _runFinished],
        [_runFinished],
      ]);
      final state = ClientToolsPageState(
        endpoint: _clientToolsEndpoint(),
        service: service,
      );
      addTearDown(state.dispose);
      await state.sendMessage('keep calculating');
      expect(
        service.calls,
        9,
        reason: 'initial request plus eight follow-up runs',
      );
      expect(_countType(state.messages, ChatMessageType.tool), 8);
      expect(
        state.messages.any((m) => m.content.contains('Stopped after 8')),
        isTrue,
      );
      expect(state.busy, isFalse);
      await state.sendMessage('try a new turn');
      expect(service.calls, 10);
      final stopped = service.histories.last
          .whereType<ToolMessage>()
          .singleWhere((m) => m.toolCallId == 'c8');
      expect(jsonDecode(stopped.content)['error']['code'], 'exchange_stopped');
    },
  );

  test(
    'a proposal on an interrupted run never executes and later turn recovers',
    () async {
      final service = FakeAgUiService([
        [_snapshotWithToolCall('interrupted-call', '6*7')],
        [_runFinished],
      ]);
      final state = ClientToolsPageState(
        endpoint: _clientToolsEndpoint(),
        service: service,
      );
      addTearDown(state.dispose);
      await state.sendMessage('calculate');
      expect(service.calls, 1);
      expect(_countType(state.messages, ChatMessageType.tool), 0);
      expect(
        state.messages.any((m) => m.content.contains('interrupted')),
        isTrue,
      );
      expect(state.busy, isFalse);
      await state.sendMessage('recover');
      expect(service.calls, 2);
      expect(
        service.histories.last.whereType<ToolMessage>().single.toolCallId,
        'interrupted-call',
      );
    },
  );

  for (final approved in [true, false]) {
    test('approval $approved waits then resumes exactly once', () async {
      final endpoint = EndpointConfig.availableEndpoints.singleWhere(
        (e) => e.path == 'human_in_the_loop',
      );
      final service = FakeAgUiService([
        [
          MessagesSnapshotEvent(
            messages: [
              AssistantMessage(
                id: 'approval-owner',
                toolCalls: [
                  ToolCall(
                    id: 'approval-call',
                    function: FunctionCall(
                      name: 'request_approval',
                      arguments: '{"summary":"Local demo","action":"demo"}',
                    ),
                  ),
                ],
              ),
            ],
          ),
          _runFinished,
        ],
        [_runFinished],
      ]);
      final state = ClientToolsPageState(endpoint: endpoint, service: service);
      addTearDown(state.dispose);
      final exchange = state.sendMessage('request approval');
      await pumpEventQueue();
      expect(service.calls, 1);
      expect(state.pendingApproval, 'Local demo');
      await state.sendMessage('duplicate send while waiting');
      expect(service.calls, 1);
      if (approved) {
        state.approve();
        state.approve();
      } else {
        state.deny();
        state.deny();
      }
      await exchange;
      expect(service.calls, 2);
      final result = service.histories.last.whereType<ToolMessage>().single;
      expect(result.toolCallId, 'approval-call');
      expect(jsonDecode(result.content)['approved'], approved);
      expect(state.pendingApproval, isNull);
      expect(state.busy, isFalse);
    });
  }

  test('card snapshots render once and unknown tools become errors', () async {
    final endpoint = EndpointConfig.availableEndpoints.singleWhere(
      (e) => e.path == 'tool_based_generative_ui',
    );
    final proposal = MessagesSnapshotEvent(
      messages: [
        AssistantMessage(
          id: 'card-owner',
          toolCalls: [
            ToolCall(
              id: 'card-call',
              function: FunctionCall(
                name: 'render_card',
                arguments:
                    '{"title":"Facts","facts":[{"label":"Answer","value":"42"}]}',
              ),
            ),
            ToolCall(
              id: 'unknown-call',
              function: FunctionCall(name: 'not_a_tool', arguments: '{}'),
            ),
          ],
        ),
      ],
    );
    final service = FakeAgUiService([
      [proposal, proposal, _runFinished],
      [proposal, _runFinished],
    ]);
    final state = ClientToolsPageState(endpoint: endpoint, service: service);
    addTearDown(state.dispose);
    await state.sendMessage('render');
    expect(service.calls, 2);
    expect(_countType(state.messages, ChatMessageType.card), 1);
    final results = service.histories.last.whereType<ToolMessage>().toList();
    expect(results, hasLength(2));
    expect(
      jsonDecode(results[1].content)['error']['code'],
      'invalid_tool_call',
    );
  });
}
