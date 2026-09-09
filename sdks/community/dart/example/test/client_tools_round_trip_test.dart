import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:ag_ui/ag_ui.dart';
import 'package:ag_ui_example/models/chat_message.dart';
import 'package:ag_ui_example/models/endpoint_config.dart';
import 'package:ag_ui_example/pages/agui_event_handling.dart';
import 'package:ag_ui_example/services/ag_ui_service.dart';
import 'package:ag_ui_example/pages/client_tools_page.dart';

/// A fake service that yields a scripted list of events per `run()` call, so the
/// round-trip loop can be driven deterministically without a server.
class FakeAgUiService extends AgUiService {
  final List<List<BaseEvent>> runs;
  final List<Object?> terminalErrors;
  int calls = 0;
  final histories = <List<Message>>[];
  final threads = <String>[];
  final definitions = <List<Tool>>[];
  final queries = <Map<String, String>>[];
  FakeAgUiService(this.runs, {this.terminalErrors = const []});

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
    final runIndex = calls;
    final events = runIndex < runs.length
        ? runs[runIndex]
        : const <BaseEvent>[];
    calls++;
    for (final e in events) {
      yield e;
    }
    if (runIndex < terminalErrors.length && terminalErrors[runIndex] != null) {
      throw terminalErrors[runIndex]!;
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
  test('child failure stays local while the root run completes', () async {
    final service = FakeAgUiService([
      [
        const SubagentStartedEvent(subagentRunId: 'child', name: 'Worker'),
        const TextMessageChunkEvent(
          messageId: 'child-output',
          subagentRunId: 'child',
          delta: 'partial',
        ),
        const SubagentErrorEvent(
          subagentRunId: 'child',
          message: 'child failed',
        ),
        const TextMessageChunkEvent(messageId: 'root', delta: 'root answer'),
        const RunFinishedEvent(
          threadId: 't',
          runId: 'r',
          outcome: RunFinishedSuccessOutcome(),
        ),
      ],
    ]);
    final state = ClientToolsPageState(
      endpoint: _clientToolsEndpoint(),
      service: service,
    );
    addTearDown(state.dispose);
    final statuses = <AgUiRunStatus>[];
    state.addListener(() => statuses.add(state.runStatus));

    await state.sendMessage('delegate');

    expect(state.busy, isFalse);
    expect(service.calls, 1);
    expect(
      state.messages
          .where((m) => m.type == ChatMessageType.system)
          .single
          .content,
      '✅ Run completed',
    );
    expect(
      state.runStatus,
      AgUiRunStatus.completed,
      reason: 'observed statuses: $statuses',
    );
    expect(state.subagents['child']!.status, AgUiSubagentStatus.failed);
    expect(
      state.messages.singleWhere((m) => m.subagentRunId == 'child').content,
      'partial',
    );
    expect(state.messages.any((m) => m.content == 'root answer'), isTrue);
  });

  test(
    'streamed child history keeps canonical identity on the next turn',
    () async {
      final service = FakeAgUiService([
        [
          const SubagentStartedEvent(subagentRunId: 'child', name: 'Worker'),
          const TextMessageChunkEvent(
            messageId: 'child-answer',
            subagentRunId: 'child',
            delta: 'first',
            metadata: {'source': 'child'},
          ),
          const RunFinishedEvent(threadId: 't', runId: 'r1'),
        ],
        [const RunFinishedEvent(threadId: 't', runId: 'r2')],
      ]);
      final state = ClientToolsPageState(
        endpoint: _clientToolsEndpoint(),
        service: service,
      );
      addTearDown(state.dispose);

      await state.sendMessage('first');
      await state.sendMessage('second');

      final child = service.histories[1]
          .whereType<AssistantMessage>()
          .singleWhere((message) => message.subagentRunId == 'child');
      expect(child.id, 'child-answer');
      expect(child.content, 'first');
      expect(child.metadata, {'source': 'child'});
    },
  );

  test(
    'colliding child tool IDs execute and render with attribution',
    () async {
      const shared = ToolCall(
        id: 'shared',
        function: FunctionCall(
          name: 'calculate',
          arguments: '{"expression":"1+1"}',
        ),
        metadata: {'kind': 'child-tool'},
      );
      final service = FakeAgUiService([
        [
          const SubagentStartedEvent(subagentRunId: 'a', name: 'Alpha'),
          const SubagentStartedEvent(subagentRunId: 'b', name: 'Beta'),
          MessagesSnapshotEvent(
            messages: [
              AssistantMessage(
                id: 'a-message',
                subagentRunId: 'a',
                toolCalls: [shared],
              ),
              AssistantMessage(
                id: 'b-message',
                subagentRunId: 'b',
                toolCalls: [shared],
              ),
            ],
          ),
          const RunFinishedEvent(threadId: 't', runId: 'r1'),
        ],
        [const RunFinishedEvent(threadId: 't', runId: 'r2')],
      ]);
      final state = ClientToolsPageState(
        endpoint: _clientToolsEndpoint(),
        service: service,
      );
      addTearDown(state.dispose);

      await state.sendMessage('calculate twice');

      final tools = state.messages.where(
        (message) => message.type == ChatMessageType.tool,
      );
      expect(tools, hasLength(2));
      expect(tools.map((message) => message.subagentRunId).toSet(), {'a', 'b'});
      expect(
        tools.every((message) => message.metadata?['kind'] == 'child-tool'),
        isTrue,
      );
      expect(
        service.histories[1]
            .whereType<ToolMessage>()
            .map((message) => message.subagentRunId)
            .toSet(),
        {'a', 'b'},
      );
    },
  );

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
    var notifications = 0;
    state.addListener(() => notifications++);

    state.sendMessage('delete the file');
    await pumpEventQueue();

    // The resolve loop is now awaiting the approval decision.
    expect(state.pendingApproval, isNotNull);

    // Disposing must complete the pending completer (false) and not throw.
    expect(() => state.dispose(), returnsNormally);
    final notificationsAtDispose = notifications;
    await pumpEventQueue();
    expect(service.calls, 1, reason: 'disposal must not launch a continuation');
    expect(
      notifications,
      notificationsAtDispose,
      reason: 'the unwound approval must not notify after disposal',
    );
  });

  test(
    'later success never promotes interrupted display fragments into history',
    () async {
      final service = FakeAgUiService([
        [
          const TextMessageStartEvent(messageId: 'interrupted-answer'),
          const TextMessageContentEvent(
            messageId: 'interrupted-answer',
            delta: 'partial answer',
          ),
          const ReasoningMessageStartEvent(messageId: 'interrupted-reasoning'),
          const ReasoningMessageContentEvent(
            messageId: 'interrupted-reasoning',
            delta: 'partial thought',
          ),
        ],
        [
          const TextMessageStartEvent(messageId: 'complete-answer'),
          const TextMessageContentEvent(
            messageId: 'complete-answer',
            delta: 'complete',
          ),
          const TextMessageEndEvent(messageId: 'complete-answer'),
          _runFinished,
        ],
        [_runFinished],
      ]);
      final state = ClientToolsPageState(
        endpoint: _clientToolsEndpoint(),
        service: service,
      );
      addTearDown(state.dispose);

      await state.sendMessage('first');
      await state.sendMessage('second');
      await state.sendMessage('third');

      expect(
        state.messages.singleWhere((m) => m.id == 'interrupted-answer').content,
        'partial answer',
      );
      expect(
        state.messages
            .singleWhere((m) => m.id == 'interrupted-reasoning')
            .content,
        'partial thought',
      );
      final thirdRunHistory = service.histories[2];
      expect(
        thirdRunHistory.any((message) => message.id == 'interrupted-answer'),
        isFalse,
      );
      expect(
        thirdRunHistory.any((message) => message.id == 'interrupted-reasoning'),
        isFalse,
      );
      expect(
        thirdRunHistory.any((message) => message.id == 'complete-answer'),
        isTrue,
      );
    },
  );

  test('thrown stream errors finalize output and allow another send', () async {
    final service = FakeAgUiService(
      [
        [
          const TextMessageStartEvent(messageId: 'errored-answer'),
          const TextMessageContentEvent(
            messageId: 'errored-answer',
            delta: 'kept',
          ),
        ],
        [_runFinished],
      ],
      terminalErrors: [StateError('transport failed')],
    );
    final state = ClientToolsPageState(
      endpoint: _clientToolsEndpoint(),
      service: service,
    );
    addTearDown(state.dispose);

    await state.sendMessage('first');
    final partial = state.messages.singleWhere(
      (message) => message.id == 'errored-answer',
    );
    expect(partial.content, 'kept');
    expect(partial.isStreaming, isFalse);
    expect(state.messages.last.content, contains('transport failed'));
    expect(state.busy, isFalse);

    await state.sendMessage('recover');
    expect(service.calls, 2);
    expect(state.busy, isFalse);
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
                      arguments:
                          '{"summary":"Approve harmless report","action":"delete_database"}',
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
      expect(state.pendingApproval, contains('Approve harmless report'));
      expect(state.pendingApproval, contains('delete_database'));
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

  for (final testCase in [
    (
      'tool_based_generative_ui',
      'render_card',
      '{"title":"Broken","facts":"not-a-list"}',
    ),
    (
      'human_in_the_loop',
      'request_approval',
      '{"summary":42,"action":"delete_database"}',
    ),
  ]) {
    test('${testCase.$2} schema errors return one structured result', () async {
      final endpoint = EndpointConfig.availableEndpoints.singleWhere(
        (item) => item.path == testCase.$1,
      );
      final service = FakeAgUiService([
        [
          MessagesSnapshotEvent(
            messages: [
              AssistantMessage(
                id: 'malformed-owner',
                toolCalls: [
                  ToolCall(
                    id: 'malformed-call',
                    function: FunctionCall(
                      name: testCase.$2,
                      arguments: testCase.$3,
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

      await state.sendMessage('run malformed tool');

      expect(service.calls, 2);
      final results = service.histories[1].whereType<ToolMessage>().where(
        (message) => message.toolCallId == 'malformed-call',
      );
      expect(results, hasLength(1));
      expect(
        jsonDecode(results.single.content)['error']['code'],
        'invalid_tool_call',
      );
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
