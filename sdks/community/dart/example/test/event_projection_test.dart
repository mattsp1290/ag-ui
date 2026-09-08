import 'dart:async';

import 'package:ag_ui/ag_ui.dart';
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
  Future<void> close() async {
    closeCalls++;
  }
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
      controller.add(const TextMessageStartEvent(messageId: 'late'));
      await controller.close();
      await Future<void>.delayed(Duration.zero);
      expect(notifications, atDispose);
      expect(service.closeCalls, 1);
    },
  );
}
