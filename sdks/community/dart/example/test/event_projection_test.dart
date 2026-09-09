import 'dart:async';

import 'package:ag_ui/ag_ui.dart';
import 'package:ag_ui_example/models/chat_message.dart';
import 'package:ag_ui_example/models/endpoint_config.dart';
import 'package:ag_ui_example/pages/agui_event_handling.dart';
import 'package:ag_ui_example/pages/chat_page.dart';
import 'package:ag_ui_example/services/ag_ui_service.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/testing.dart';
import 'package:http/http.dart' as http;

class _Projection extends ChangeNotifier with AgUiEventHandling {}

class _FakeService extends AgUiService {
  final List<Stream<BaseEvent>> runs;
  final StreamController<ConnectionStatus> statuses =
      StreamController<ConnectionStatus>.broadcast();
  int closeCalls = 0;

  _FakeService(this.runs)
    : super(httpClient: MockClient((_) async => http.Response('', 200)));

  @override
  Stream<BaseEvent> run(
    String endpoint, {
    required String threadId,
    required List<Message> messages,
    List<Tool> tools = const [],
    dynamic state,
    Map<String, String> extraQuery = const {},
  }) => runs.removeAt(0);

  @override
  Stream<ConnectionStatus> get connectionStatus => statuses.stream;

  @override
  Future<void> close() async {
    closeCalls++;
  }
}

Stream<BaseEvent> _partialThenError() async* {
  yield const TextMessageStartEvent(messageId: 'partial-answer');
  yield const TextMessageContentEvent(
    messageId: 'partial-answer',
    delta: 'visible',
  );
  yield const ReasoningMessageStartEvent(messageId: 'partial-reasoning');
  yield const ReasoningMessageContentEvent(
    messageId: 'partial-reasoning',
    delta: 'because',
  );
  throw StateError('transport failed');
}

const _endpoint = EndpointConfig(
  name: 'Chat',
  path: 'agentic_chat',
  description: 'Test',
  icon: Icons.chat,
);

void main() {
  test('stream and snapshot reconcile text and reasoning by protocol ID', () {
    final projection = _Projection()..beginRun();
    projection.handleCommonEvent(
      const TextMessageStartEvent(messageId: 'answer'),
    );
    projection.handleCommonEvent(
      const TextMessageContentEvent(messageId: 'answer', delta: 'hel'),
    );
    projection.handleCommonEvent(
      const ReasoningMessageStartEvent(messageId: 'reason'),
    );
    projection.handleCommonEvent(
      const ReasoningMessageContentEvent(messageId: 'reason', delta: 'why'),
    );
    projection.reconcileSnapshot(const [
      AssistantMessage(id: 'answer', content: 'hello'),
      ReasoningMessage(id: 'reason', content: 'why'),
    ]);
    expect(
      projection.messages.where((m) => m.id == 'answer').single.content,
      'hello',
    );
    expect(
      projection.messages.where((m) => m.id == 'reason').single.content,
      'why',
    );
    expect(projection.messages.length, 2);
  });

  test('run error is terminal and trailing content is ignored', () {
    final projection = _Projection()..beginRun();
    projection.handleCommonEvent(
      const TextMessageStartEvent(messageId: 'answer'),
    );
    projection.handleCommonEvent(
      const TextMessageContentEvent(messageId: 'answer', delta: 'kept'),
    );
    projection.handleCommonEvent(const RunErrorEvent(message: 'failed'));
    projection.handleCommonEvent(
      const TextMessageContentEvent(messageId: 'answer', delta: 'ignored'),
    );
    expect(projection.runIsTerminal, isTrue);
    expect(projection.messages.first.content, 'kept');
    expect(projection.messages.first.isStreaming, isFalse);
    expect(projection.messages.last.content, contains('failed'));
  });

  test(
    'ChatPageState ignores trailing events and recovers after error/interruption',
    () async {
      final service = _FakeService([
        Stream<BaseEvent>.fromIterable(const [
          TextMessageStartEvent(messageId: 'a'),
          TextMessageContentEvent(messageId: 'a', delta: 'kept'),
          RunErrorEvent(message: 'failed'),
          TextMessageContentEvent(messageId: 'a', delta: 'ignored'),
        ]),
        const Stream<BaseEvent>.empty(),
      ]);
      final state = ChatPageState(endpoint: _endpoint, service: service);
      state.sendMessage('first');
      await Future<void>.delayed(Duration.zero);
      await Future<void>.delayed(Duration.zero);
      expect(state.messages.where((m) => m.id == 'a').single.content, 'kept');
      expect(state.messages.last.content, contains('failed'));
      expect(state.isLoading, isFalse);

      state.sendMessage('second');
      await Future<void>.delayed(Duration.zero);
      await Future<void>.delayed(Duration.zero);
      expect(state.messages.last.content, contains('before the run finished'));
      expect(state.isLoading, isFalse);
      state.dispose();
      await Future<void>.delayed(Duration.zero);
      expect(service.closeCalls, 1);
    },
  );

  test(
    'ChatPageState disposal during a run causes no late notifications',
    () async {
      final controller = StreamController<BaseEvent>();
      final service = _FakeService([controller.stream]);
      final state = ChatPageState(endpoint: _endpoint, service: service);
      var notifications = 0;
      state.addListener(() => notifications++);
      state.sendMessage('hello');
      await Future<void>.delayed(Duration.zero);
      state.dispose();
      final atDispose = notifications;
      final statusAtDispose = state.connectionStatus;
      service.statuses.add(ConnectionStatus.connected);
      controller.add(const TextMessageStartEvent(messageId: 'late'));
      await controller.close();
      await Future<void>.delayed(Duration.zero);
      expect(notifications, atDispose);
      expect(state.connectionStatus, statusAtDispose);
      expect(service.closeCalls, 1);
      await service.statuses.close();
    },
  );

  test('transport errors finalize partial text and reasoning', () async {
    final service = _FakeService([_partialThenError()]);
    final state = ChatPageState(endpoint: _endpoint, service: service);
    addTearDown(() async {
      state.dispose();
      await service.statuses.close();
    });

    state.sendMessage('hello');
    await Future<void>.delayed(Duration.zero);
    await Future<void>.delayed(Duration.zero);

    expect(state.isLoading, isFalse);
    expect(
      state.messages.where((message) => message.id == 'partial-answer').single,
      isA<ChatMessage>()
          .having((message) => message.content, 'content', 'visible')
          .having((message) => message.isStreaming, 'isStreaming', isFalse),
    );
    expect(
      state.messages
          .where((message) => message.id == 'partial-reasoning')
          .single,
      isA<ChatMessage>()
          .having((message) => message.content, 'content', 'because')
          .having((message) => message.isStreaming, 'isStreaming', isFalse),
    );
    expect(state.messages.last.content, contains('transport failed'));
  });

  test(
    'tool lifecycle and cumulative snapshot retain one settled row',
    () async {
      const toolCall = ToolCall(
        id: 'tool-1',
        function: FunctionCall(
          name: 'calculate',
          arguments: '{"expression":"2+2"}',
        ),
      );
      final service = _FakeService([
        Stream<BaseEvent>.fromIterable([
          const ToolCallStartEvent(
            toolCallId: 'tool-1',
            toolCallName: 'calculate',
          ),
          const ToolCallArgsEvent(
            toolCallId: 'tool-1',
            delta: '{"expression":',
          ),
          const ToolCallArgsEvent(toolCallId: 'tool-1', delta: '"2+2"}'),
          const ToolCallEndEvent(toolCallId: 'tool-1'),
          const ToolCallResultEvent(
            messageId: 'tool-result-1',
            toolCallId: 'tool-1',
            content: '{"result":4}',
          ),
          MessagesSnapshotEvent(
            messages: [
              AssistantMessage(id: 'assistant-1', toolCalls: [toolCall]),
            ],
          ),
          const RunFinishedEvent(threadId: 'thread', runId: 'run'),
        ]),
      ]);
      final state = ChatPageState(endpoint: _endpoint, service: service);
      addTearDown(() async {
        state.dispose();
        await service.statuses.close();
      });

      state.sendMessage('calculate');
      await Future<void>.delayed(Duration.zero);
      await Future<void>.delayed(Duration.zero);

      final tool = state.messages
          .where((message) => message.id == 'tool-1')
          .single;
      expect(tool.toolName, 'calculate');
      expect(tool.toolArgs, '{"expression":"2+2"}');
      expect(tool.toolResult, '{"result":4}');
      expect(tool.isStreaming, isFalse);
    },
  );

  test('no-ID chunks stay isolated by subagent and merge late metadata', () {
    final projection = _Projection()..beginRun();
    projection.handleCommonEvent(
      const SubagentStartedEvent(subagentRunId: 'a', name: 'Alpha'),
    );
    projection.handleCommonEvent(
      const SubagentStartedEvent(subagentRunId: 'b', name: 'Beta'),
    );
    projection.handleCommonEvent(
      const TextMessageChunkEvent(
        subagentRunId: 'a',
        delta: 'A1',
        metadata: {'early': true},
      ),
    );
    projection.handleCommonEvent(
      const TextMessageChunkEvent(subagentRunId: 'b', delta: 'B1'),
    );
    projection.handleCommonEvent(
      const TextMessageChunkEvent(
        subagentRunId: 'a',
        delta: 'A2',
        metadata: {'late': true},
      ),
    );

    final alpha = projection.messages.singleWhere(
      (message) => message.subagentRunId == 'a',
    );
    final beta = projection.messages.singleWhere(
      (message) => message.subagentRunId == 'b',
    );
    expect(alpha.content, 'A1A2');
    expect(alpha.metadata, {'early': true, 'late': true});
    expect(alpha.subagentName, 'Alpha');
    expect(beta.content, 'B1');
    expect(beta.subagentName, 'Beta');
    expect(alpha.id, isNot(beta.id));
  });

  test(
    'snapshot identity includes attribution and preserves typed metadata',
    () {
      final projection = _Projection()..beginRun();
      projection.reconcileSnapshot(const [
        AssistantMessage(
          id: 'same',
          content: 'alpha',
          subagentRunId: 'a',
          metadata: {'source': 'a'},
        ),
        AssistantMessage(
          id: 'same',
          content: 'beta',
          subagentRunId: 'b',
          metadata: {'source': 'b'},
        ),
      ]);

      expect(projection.messages, hasLength(2));
      expect(
        projection.messages.singleWhere((m) => m.subagentRunId == 'a').metadata,
        {'source': 'a'},
      );
      expect(
        projection.messages.singleWhere((m) => m.subagentRunId == 'b').content,
        'beta',
      );
    },
  );

  test('child failure does not terminate root and terminal state is typed', () {
    final projection = _Projection()..beginRun();
    projection.handleCommonEvent(
      const SubagentStartedEvent(subagentRunId: 'child', name: 'Worker'),
    );
    projection.handleCommonEvent(
      const SubagentErrorEvent(subagentRunId: 'child', message: 'failed'),
    );
    expect(projection.runIsTerminal, isFalse);
    expect(projection.subagents['child']!.status, AgUiSubagentStatus.failed);

    projection.handleCommonEvent(
      const TextMessageChunkEvent(messageId: 'root', delta: 'still running'),
    );
    projection.handleCommonEvent(
      RunFinishedEvent(
        threadId: 'thread',
        runId: 'run',
        outcome: RunFinishedInterruptOutcome(
          interrupts: const [Interrupt(id: 'approval', reason: 'review')],
        ),
        usage: [TokenUsage(inputTokens: 3, outputTokens: 2, totalTokens: 5)],
      ),
    );

    expect(projection.runIsTerminal, isTrue);
    expect(projection.runIsAwaitingInput, isTrue);
    expect(projection.runStatus, AgUiRunStatus.awaitingInput);
    expect(projection.runUsage.single.totalTokens, 5);
    expect(projection.messages.last.content, '⏸️ Run awaiting input');
  });

  test('suspended child and root error usage reset on the next run', () {
    final projection = _Projection()..beginRun();
    projection.handleCommonEvent(
      const SubagentFinishedEvent(
        subagentRunId: 'ancestor',
        outcome: SubagentFinishedSuspendedOutcome(),
      ),
    );
    expect(
      projection.subagents['ancestor']!.status,
      AgUiSubagentStatus.suspended,
    );
    expect(projection.subagents['ancestor']!.interruptIds, isNull);

    projection.handleCommonEvent(
      RunErrorEvent(
        message: 'root failed',
        usage: [TokenUsage(inputTokens: 7)],
      ),
    );
    expect(projection.runStatus, AgUiRunStatus.error);
    expect(projection.runUsage.single.inputTokens, 7);

    projection.beginRun(resumedSubagentIds: const ['ancestor']);
    expect(projection.runStatus, AgUiRunStatus.running);
    expect(projection.runUsage, isEmpty);
    expect(projection.subagents.keys, ['ancestor']);
    expect(
      projection.subagents['ancestor']!.status,
      AgUiSubagentStatus.running,
    );
    expect(projection.subagents['ancestor']!.interruptIds, isNull);
    expect(projection.runIsTerminal, isFalse);
  });
}
