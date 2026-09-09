import 'dart:async';
import 'dart:convert';

import 'package:ag_ui/ag_ui.dart';
import 'package:ag_ui_example/services/ag_ui_service.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

class _CountingClient extends http.BaseClient {
  final List<http.BaseRequest> requests = [];
  int closeCount = 0;

  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) async {
    requests.add(request);
    return http.StreamedResponse(
      const Stream<List<int>>.empty(),
      200,
      headers: {'content-type': 'text/event-stream'},
    );
  }

  @override
  void close() {
    closeCount++;
  }
}

class _PendingClient extends http.BaseClient {
  final response = Completer<http.StreamedResponse>();
  int closeCount = 0;
  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) =>
      response.future;
  @override
  void close() {
    closeCount++;
  }
}

class _StreamingClient extends http.BaseClient {
  final Stream<List<int>> body;
  int closeCount = 0;
  _StreamingClient(this.body);
  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) async =>
      http.StreamedResponse(
        body,
        200,
        headers: {'content-type': 'text/event-stream'},
      );
  @override
  void close() {
    closeCount++;
  }
}

class _ThrowingCloseClient extends _CountingClient {
  @override
  void close() {
    super.close();
    throw StateError('close failed');
  }
}

class _CancelRetryClient extends http.BaseClient {
  final never = Completer<http.StreamedResponse>();
  int sends = 0;

  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) {
    sends++;
    if (sends == 1) return never.future;
    return Future.value(http.StreamedResponse(const Stream.empty(), 200));
  }
}

class _StreamingRetryClient extends http.BaseClient {
  final firstListened = Completer<void>();
  final firstCancelled = Completer<void>();
  late final StreamController<List<int>> firstBody;
  int sends = 0;

  _StreamingRetryClient() {
    firstBody = StreamController<List<int>>(
      onListen: () => firstListened.complete(),
      onCancel: () => firstCancelled.complete(),
    );
  }

  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) async {
    sends++;
    return http.StreamedResponse(
      sends == 1 ? firstBody.stream : const Stream.empty(),
      200,
      headers: {'content-type': 'text/event-stream'},
    );
  }
}

class _LatePendingThenStreamingClient extends http.BaseClient {
  final firstResponse = Completer<http.StreamedResponse>();
  final secondListened = Completer<void>();
  final secondCancelled = Completer<void>();
  late final StreamController<List<int>> secondBody;
  int sends = 0;

  _LatePendingThenStreamingClient() {
    secondBody = StreamController<List<int>>(
      onListen: () => secondListened.complete(),
      onCancel: () => secondCancelled.complete(),
    );
  }

  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) {
    sends++;
    if (sends == 1) return firstResponse.future;
    return Future.value(
      http.StreamedResponse(
        secondBody.stream,
        200,
        headers: {'content-type': 'text/event-stream'},
      ),
    );
  }
}

void main() {
  test(
    'canceling a pending send releases ownership before HTTP completion',
    () async {
      final client = _CancelRetryClient();
      final service = AgUiService(httpClient: client);
      final subscription = service
          .run('first', threadId: 't1', messages: const [])
          .listen((_) {}, onError: (_) {});
      await Future<void>.delayed(Duration.zero);
      await subscription.cancel().timeout(const Duration(seconds: 1));
      expect(service.isBusy, isFalse);
      await service
          .run('retry', threadId: 't2', messages: const [])
          .drain<void>()
          .timeout(const Duration(seconds: 1));
      expect(client.sends, 2);
      await service.close();
    },
  );

  test(
    'canceling active SSE suppresses late events and permits retry',
    () async {
      final client = _StreamingRetryClient();
      final service = AgUiService(httpClient: client);
      final events = <BaseEvent>[];
      final subscription = service
          .run('first', threadId: 't1', messages: const [])
          .listen(events.add, onError: (_) {});
      await client.firstListened.future.timeout(const Duration(seconds: 1));
      try {
        await subscription.cancel().timeout(const Duration(seconds: 1));
        await client.firstCancelled.future.timeout(const Duration(seconds: 1));
        client.firstBody.add(
          utf8.encode(
            'data: {"type":"RUN_STARTED","threadId":"t","runId":"late"}\n\n',
          ),
        );
        await Future<void>.delayed(Duration.zero);
        expect(events, isEmpty);
        expect(service.isBusy, isFalse);
        await service
            .run('retry', threadId: 't2', messages: const [])
            .drain<void>();
        expect(client.sends, 2);
      } finally {
        await client.firstBody.close();
        await service.close();
      }
    },
  );

  test(
    'a late canceled response cannot take ownership from its retry',
    () async {
      final client = _LatePendingThenStreamingClient();
      final service = AgUiService(httpClient: client);
      final first = service
          .run('first', threadId: 't1', messages: const [])
          .listen((_) {}, onError: (_) {});
      await Future<void>.delayed(Duration.zero);
      await first.cancel().timeout(const Duration(seconds: 1));

      final events = <BaseEvent>[];
      final second = service
          .run('second', threadId: 't2', messages: const [])
          .listen(events.add, onError: (_) {});
      await client.secondListened.future.timeout(const Duration(seconds: 1));
      try {
        client.firstResponse.complete(
          http.StreamedResponse(const Stream.empty(), 200),
        );
        await Future<void>.delayed(Duration.zero);
        expect(service.isBusy, isTrue);
        expect(client.secondCancelled.isCompleted, isFalse);

        client.secondBody.add(
          utf8.encode(
            'data: {"type":"RUN_STARTED","threadId":"t2","runId":"r2"}\n\n',
          ),
        );
        await Future<void>.delayed(Duration.zero);
        expect(events, hasLength(1));

        await second.cancel().timeout(const Duration(seconds: 1));
        await client.secondCancelled.future.timeout(const Duration(seconds: 1));
        expect(service.isBusy, isFalse);
      } finally {
        await client.secondBody.close();
        await service.close();
      }
    },
  );

  test('an unlistened cold stream does not reserve the service', () async {
    final client = _CountingClient();
    final service = AgUiService(httpClient: client);
    service.run('unused', threadId: 'cold', messages: const []);
    expect(service.isBusy, isFalse);
    await service
        .run('agentic_chat', threadId: 'active', messages: const [])
        .drain<void>();
    expect(client.requests, hasLength(1));
    await service.close();
  });

  test('two cold streams claim one active exchange when listened', () async {
    final client = _PendingClient();
    final service = AgUiService(httpClient: client);
    final first = service.run('first', threadId: 't1', messages: const []);
    final second = service.run('second', threadId: 't2', messages: const []);
    final firstDone = first.drain<void>().catchError((Object _) {});
    await Future<void>.delayed(Duration.zero);
    await expectLater(second, emitsError(isA<StateError>()));
    client.response.complete(http.StreamedResponse(const Stream.empty(), 200));
    await firstDone;
    await service.close();
  });

  test(
    'a stream prepared before close cannot send when later listened',
    () async {
      final client = _CountingClient();
      final service = AgUiService(httpClient: client);
      final cold = service.run(
        'agentic_chat',
        threadId: 't',
        messages: const [],
      );
      await service.close();
      await expectLater(cold, emitsError(isA<StateError>()));
      expect(client.requests, isEmpty);
    },
  );

  test(
    'status controller closes even when the SDK HTTP client close fails',
    () async {
      final client = _ThrowingCloseClient();
      final service = AgUiService(httpClient: client);
      var statusDone = false;
      final subscription = service.connectionStatus.listen(
        (_) {},
        onDone: () => statusDone = true,
      );
      await expectLater(service.close(), throwsStateError);
      await Future<void>.delayed(Duration.zero);
      expect(statusDone, isTrue);
      expect(client.closeCount, 1);
      await subscription.cancel();
    },
  );

  test(
    'default URL is used by the actual HTTP request and client closes once',
    () async {
      final client = _CountingClient();
      final service = AgUiService(httpClient: client);
      await service
          .run(
            'agentic_chat',
            threadId: 't',
            messages: [UserMessage(id: 'u', content: 'hi')],
          )
          .drain<void>();
      expect(
        client.requests.single.url.toString(),
        'http://127.0.0.1:8080/agentic_chat',
      );
      await service.close();
      await service.close();
      service.dispose();
      expect(client.closeCount, 1);
      expect(
        () => service.run('agentic_chat', threadId: 't', messages: const []),
        throwsStateError,
      );
    },
  );

  test(
    'normalizes URL, encodes query, snapshots state, and creates fresh run IDs',
    () async {
      final requests = <http.Request>[];
      final client = MockClient((request) async {
        requests.add(request);
        return http.Response(
          '',
          200,
          headers: {'content-type': 'text/event-stream'},
        );
      });
      final service = AgUiService(
        baseUrl: 'http://localhost:8080///',
        httpClient: client,
      );
      expect(service.baseUrl, 'http://localhost:8080');

      final state = <String, dynamic>{
        'recipe': <String, dynamic>{'servings': 2},
      };
      final parts = <InputContent>[const TextInputContent('hello')];
      final toolSchema = <String, dynamic>{
        'type': 'object',
        'properties': <String, dynamic>{
          'value': <String, dynamic>{'type': 'string'},
        },
      };
      final history = <Message>[
        UserMessage.multimodal(id: 'user-1', parts: parts),
      ];
      final firstRun = service.run(
        'human_in_the_loop',
        threadId: 'thread',
        messages: history,
        tools: [
          Tool(
            name: 'nested',
            description: 'snapshot test',
            parameters: toolSchema,
          ),
        ],
        state: state,
        extraQuery: {'approval': 'off & later'},
      );
      state['recipe'] = {'servings': 99};
      history.add(UserMessage(id: 'too-late', content: 'mutation'));
      parts.add(const TextInputContent('late nested part'));
      (toolSchema['properties'] as Map<String, dynamic>).clear();
      await firstRun.drain<void>();
      await service
          .run(
            'agentic_chat',
            threadId: 'thread',
            messages: [UserMessage(id: 'user-2', content: 'again')],
          )
          .drain<void>();

      expect(
        requests.first.url.toString(),
        'http://localhost:8080/human_in_the_loop?approval=off+%26+later',
      );
      final first = jsonDecode(requests.first.body) as Map<String, dynamic>;
      final second = jsonDecode(requests.last.body) as Map<String, dynamic>;
      expect((first['state'] as Map<String, dynamic>)['recipe'], {
        'servings': 2,
      });
      expect(first['messages'], hasLength(1));
      expect(
        ((first['messages'] as List).single as Map)['content'],
        hasLength(1),
      );
      expect(
        ((((first['tools'] as List).single as Map)['parameters']
                as Map)['properties']
            as Map),
        contains('value'),
      );
      expect(first['runId'], isNot(second['runId']));
      await service.close();
      await service.close();
    },
  );

  test(
    'an asynchronous transport error releases ownership for retry',
    () async {
      var sends = 0;
      final client = MockClient((_) async {
        sends++;
        return http.Response(
          '',
          sends == 1 ? 503 : 200,
          headers: {'content-type': 'text/event-stream'},
        );
      });
      final service = AgUiService(httpClient: client);
      final statuses = <ConnectionStatus>[];
      final subscription = service.connectionStatus.listen(statuses.add);
      await expectLater(
        service.run('first', threadId: 't1', messages: const []),
        emitsError(isA<AGUIError>()),
      );
      expect(service.isBusy, isFalse);
      expect(statuses, contains(ConnectionStatus.error));
      await service
          .run('retry', threadId: 't2', messages: const [])
          .drain<void>();
      expect(sends, 2);
      await subscription.cancel();
      await service.close();
    },
  );

  test(
    'preflight serialization and endpoint failures do not leave service busy',
    () async {
      final service = AgUiService(
        httpClient: MockClient((_) async => http.Response('', 200)),
      );
      expect(
        () => service.run(
          'agentic_chat',
          threadId: 't',
          messages: const [],
          state: {'bad': Object()},
        ),
        throwsA(isA<JsonUnsupportedObjectError>()),
      );
      expect(service.isBusy, isFalse);
      expect(
        () => service.run('', threadId: 't', messages: const []),
        throwsArgumentError,
      );
      expect(service.isBusy, isFalse);
      await service
          .run('agentic_chat', threadId: 't', messages: const [])
          .drain<void>();
      await service.close();
    },
  );

  test(
    'close cancels a pending HTTP send without late status events',
    () async {
      final client = _PendingClient();
      final service = AgUiService(httpClient: client);
      final statuses = <ConnectionStatus>[];
      final statusSub = service.connectionStatus.listen(statuses.add);
      final runDone = service
          .run('agentic_chat', threadId: 't', messages: const [])
          .drain<void>()
          .catchError((Object _) {});
      await Future<void>.delayed(Duration.zero);
      await service.close();
      expect(service.isBusy, isFalse);
      final countAtClose = statuses.length;
      client.response.complete(
        http.StreamedResponse(const Stream.empty(), 200),
      );
      await runDone;
      await Future<void>.delayed(Duration.zero);
      expect(statuses, hasLength(countAtClose));
      expect(client.closeCount, 1);
      await statusSub.cancel();
    },
  );

  test(
    'close during an active response stream suppresses late events',
    () async {
      final bytes = StreamController<List<int>>();
      final streamingClient = _StreamingClient(bytes.stream);
      final service = AgUiService(httpClient: streamingClient);
      final received = <BaseEvent>[];
      final subscription = service
          .run('agentic_chat', threadId: 't', messages: const [])
          .listen(received.add, onError: (_) {});
      await Future<void>.delayed(Duration.zero);
      await service.close();
      expect(service.isBusy, isFalse);
      bytes.add(
        utf8.encode(
          'data: {"type":"RUN_STARTED","threadId":"t","runId":"r"}\n\n',
        ),
      );
      await bytes.close();
      await Future<void>.delayed(Duration.zero);
      expect(received, isEmpty);
      expect(streamingClient.closeCount, 1);
      await subscription.cancel();
    },
  );

  test(
    'rejects invalid configuration and duplicate active exchanges',
    () async {
      for (final value in [
        '',
        'ftp://localhost',
        'not a url',
        'http://host/?q=x',
        'http://:8080',
      ]) {
        expect(() => AgUiService(baseUrl: value), throwsArgumentError);
      }
      final response = Completer<http.Response>();
      final service = AgUiService(
        httpClient: MockClient((_) => response.future),
      );
      final first = service
          .run(
            'agentic_chat',
            threadId: 't',
            messages: [UserMessage(id: 'u', content: 'one')],
          )
          .listen((_) {});
      await Future<void>.delayed(Duration.zero);
      expect(
        () => service
            .run(
              'agentic_chat',
              threadId: 't',
              messages: [UserMessage(id: 'u2', content: 'two')],
            )
            .drain<void>(),
        throwsStateError,
      );
      response.complete(http.Response('', 200));
      await first.asFuture<void>();
      await service.close();
    },
  );

  test('service decodes subagent attribution and terminal usage', () async {
    final payloads = [
      const SubagentStartedEvent(subagentRunId: 'child', name: 'Worker'),
      const TextMessageChunkEvent(
        messageId: 'answer',
        subagentRunId: 'child',
        delta: 'done',
      ),
      RunFinishedEvent(
        threadId: 'thread',
        runId: 'run',
        outcome: const RunFinishedSuccessOutcome(),
        usage: [TokenUsage(inputTokens: 2, outputTokens: 1, totalTokens: 3)],
      ),
    ];
    final body = payloads
        .map((event) => 'data: ${jsonEncode(event.toJson())}\n\n')
        .join();
    final service = AgUiService(
      httpClient: MockClient(
        (_) async => http.Response(
          body,
          200,
          headers: {'content-type': 'text/event-stream'},
        ),
      ),
    );
    addTearDown(service.close);

    final events = await service
        .run('agentic_chat', threadId: 'thread', messages: const [])
        .toList();

    expect(events[0], isA<SubagentStartedEvent>());
    expect((events[1] as TextMessageChunkEvent).subagentRunId, 'child');
    final finished = events[2] as RunFinishedEvent;
    expect(finished.outcome, isA<RunFinishedSuccessOutcome>());
    expect(finished.usage!.single.totalTokens, 3);
  });
}
