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

void main() {
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
      final history = <Message>[UserMessage(id: 'user-1', content: 'hello')];
      final firstRun = service.run(
        'human_in_the_loop',
        threadId: 'thread',
        messages: history,
        state: state,
        extraQuery: {'approval': 'off & later'},
      );
      state['recipe'] = {'servings': 99};
      history.add(UserMessage(id: 'too-late', content: 'mutation'));
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
      expect(first['runId'], isNot(second['runId']));
      await service.close();
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
}
