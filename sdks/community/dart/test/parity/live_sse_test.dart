import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:ag_ui/ag_ui.dart';
import 'package:test/test.dart';

import '../../tool/parity_support.dart';

Future<void> _writeFragmented(HttpResponse response, String payload) async {
  response.headers.contentType = ContentType('text', 'event-stream');
  response.headers.set('cache-control', 'no-cache');
  final bytes = utf8.encode(payload);
  for (var index = 0; index < bytes.length; index++) {
    response.add([bytes[index]]);
    if (index.isEven) {
      await response.flush();
    }
  }
  await response.close();
}

void main() {
  test('loopback client preserves mixed fragmented SSE and resume requests',
      () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    final bodies = <Map<String, dynamic>>[];
    final handlerErrors = <Object>[];
    final cancelRequestSeen = Completer<void>();
    final releaseCancelResponse = Completer<void>();
    var requestCount = 0;
    final serverSubscription = server.listen((request) async {
      try {
        requestCount++;
        bodies.add(
          (jsonDecode(await utf8.decoder.bind(request).join()) as Map)
              .cast<String, dynamic>(),
        );
        switch (requestCount) {
          case 1:
            final frames = <String>[
              ': keep-alive\r\n\r\n',
              'data: {"type":"SUBAGENT_STARTED","subagentRunId":"child","name":"worker"}\r\r',
              'data: {"type":"SUBAGENT_ERROR","subagentRunId":"sibling","message":"failed"}\n\n',
              'data: {"type":"TEXT_MESSAGE_START","messageId":"message","role":"assistant"}\r\n\r\n',
              'data: {"type":"TEXT_MESSAGE_CONTENT","messageId":"message","delta":"café 🚀"}\r\r',
              'data: {"type":"RUN_FINISHED","threadId":"thread","runId":"simple","outcome":{"type":"interrupt","interrupts":[{"id":"approval","reason":"approve","subagentRunId":"child"}]},"usage":[{"provider":"","inputTokens":0,"outputTokens":2}]}\n\n',
            ].join();
            await _writeFragmented(request.response, frames);
          case 2:
            await _writeFragmented(
              request.response,
              'data: {"type":"RUN_FINISHED","threadId":"thread","runId":"canonical","outcome":{"type":"success"}}\r\n\r\n',
            );
          case 3:
            await _writeFragmented(
              request.response,
              [
                'data: {"type":"TEXT_MESSAGE_START","messageId":"partial","role":"assistant"}\n\n',
                'data: {"type":"TEXT_MESSAGE_CONTENT","messageId":"partial","delta":"visible"}\n\n',
                'data: {"type":"RUN_ERROR","message":"root failed","code":"root-failed","usage":[{"inputTokens":3}]}\n\n',
              ].join(),
            );
          case 4:
            cancelRequestSeen.complete();
            await releaseCancelResponse.future;
            await _writeFragmented(
              request.response,
              'data: {"type":"RUN_ERROR","message":"late"}\n\n',
            );
          default:
            request.response.statusCode = HttpStatus.notFound;
            await request.response.close();
        }
      } on Object catch (error) {
        handlerErrors.add(error);
        try {
          await request.response.close();
        } on Object {
          // The cancellation case may already have closed its socket.
        }
      }
    });
    final client = AgUiClient(
      config: AgUiClientConfig(
        baseUrl: 'http://${server.address.host}:${server.port}',
        maxRetries: 0,
        requestTimeout: const Duration(seconds: 5),
        connectionTimeout: const Duration(seconds: 5),
      ),
    );

    try {
      final first = await client
          .runAgent(
            'stream',
            const SimpleRunAgentInput(
              threadId: 'thread',
              runId: 'simple',
              messages: [],
              tools: [],
              context: [],
            ),
          )
          .toList();
      expect(first.map((event) => event.runtimeType), [
        SubagentStartedEvent,
        SubagentErrorEvent,
        TextMessageStartEvent,
        TextMessageContentEvent,
        RunFinishedEvent,
      ]);
      expect((first[3] as TextMessageContentEvent).delta, 'café 🚀');
      final terminal = first.last as RunFinishedEvent;
      expect(terminal.outcome, isA<RunFinishedInterruptOutcome>());
      final interruptOutcome = terminal.outcome! as RunFinishedInterruptOutcome;
      expect(
        interruptOutcome.interrupts.single.subagentRunId,
        'child',
      );
      expect(terminal.usage?.single.toJson(), {
        'provider': '',
        'inputTokens': 0,
        'outputTokens': 2,
      });
      expect(first.whereType<RunFinishedEvent>(), hasLength(1));
      expect(first.whereType<RunErrorEvent>(), isEmpty);

      Object? explicitNull;
      final resumed = await client
          .runAgentInput(
            'stream',
            RunAgentInput(
              threadId: 'thread',
              runId: 'canonical',
              state: {
                'nested': [null, false, 0],
              },
              messages: const [],
              tools: const [],
              context: const [],
              forwardedProps: explicitNull,
              resume: const [
                ResumeEntry(
                  interruptId: 'approval',
                  status: ResumeStatus.resolved,
                  payload: {
                    'approved': false,
                    'editedArgs': <String, dynamic>{},
                  },
                  metadata: {'subagentRunId': 'child'},
                ),
              ],
            ),
          )
          .toList();
      expect(resumed, hasLength(1));
      expect(
        (resumed.single as RunFinishedEvent).outcome,
        isA<RunFinishedSuccessOutcome>(),
      );

      expect(bodies, hasLength(2));
      expect(bodies.first, containsPair('state', <String, dynamic>{}));
      expect(bodies.first, containsPair('forwardedProps', <String, dynamic>{}));
      final canonical = bodies[1];
      expect(canonical['state'], {
        'nested': [null, false, 0],
      });
      expect(canonical.containsKey('forwardedProps'), isTrue);
      expect(canonical['forwardedProps'], isNull);
      final resume = (canonical['resume'] as List).single as Map;
      expect(resume['interruptId'], 'approval');
      expect(resume['status'], 'resolved');
      expect(resume['metadata'], {'subagentRunId': 'child'});

      final failed = await client
          .runAgent(
            'error',
            const SimpleRunAgentInput(threadId: 'thread', runId: 'error'),
          )
          .toList();
      expect(failed.map((event) => event.runtimeType), [
        TextMessageStartEvent,
        TextMessageContentEvent,
        RunErrorEvent,
      ]);
      final rootError = failed.last as RunErrorEvent;
      expect(rootError.code, 'root-failed');
      expect(rootError.usage?.single.inputTokens, 3);

      final token = CancelToken();
      final cancelled = client
          .runAgent(
            'cancel',
            const SimpleRunAgentInput(
              threadId: 'thread',
              runId: 'cancelled',
            ),
            cancelToken: token,
          )
          .toList();
      await cancelRequestSeen.future.timeout(const Duration(seconds: 5));
      token.cancel();
      await expectLater(cancelled, throwsA(isA<CancellationError>()));
      releaseCancelResponse.complete();

      expect(bodies, hasLength(4));
      expect(requestCount, 4);
      expect(handlerErrors, isEmpty);
    } finally {
      if (!releaseCancelResponse.isCompleted) {
        releaseCancelResponse.complete();
      }
      await client.close();
      await serverSubscription.cancel();
      await server.close(force: true);
    }
  });

  final goSseDirectory = Platform.environment['AG_UI_DART_PARITY_GO_SSE_DIR'];
  test(
    'actual Go-produced SSE matches the pinned scenarios',
    () async {
      final fixture = asMap(
        jsonDecode(
          File(
            '${repositoryRoot().path}/sdks/community/go/testdata/parity/sse-scenarios.json',
          ).readAsStringSync(),
        ),
        'SSE fixture',
      );
      for (final rawScenario in fixture['scenarios'] as List) {
        final scenario = asMap(rawScenario, 'SSE scenario');
        final id = scenario['id'] as String;
        final events = await EventStreamAdapter()
            .fromRawSseStream(
              Stream.value(
                File('$goSseDirectory/go-$id.sse').readAsStringSync(),
              ),
            )
            .toList();
        final expected = (scenario['events'] as List)
            .map((raw) => Map<String, dynamic>.from(raw as Map))
            .toList();
        for (final event in expected) {
          if (event['type'] == 'ACTIVITY_SNAPSHOT' &&
              event['replace'] == true) {
            event.remove('replace');
          }
        }
        expect(events, hasLength(expected.length), reason: id);
        for (var index = 0; index < events.length; index++) {
          expect(
            deepEqualJson(events[index].toJson(), expected[index]),
            isTrue,
            reason: '$id event $index',
          );
        }
      }
    },
    skip: goSseDirectory == null
        ? 'AG_UI_DART_PARITY_GO_SSE_DIR is not configured'
        : false,
  );

  test('EventStreamAdapter owns independent listeners and terminal delivery',
      () async {
    final adapter = EventStreamAdapter();
    final first = await adapter
        .fromRawSseStream(
          Stream.fromIterable([
            ': comment\r',
            '\ndata: {"type":"RUN_ERROR","message":"failed"}\r',
            '\r',
          ]),
        )
        .toList();
    final second = await adapter
        .fromRawSseStream(
          Stream.value(
            'data: {"type":"RUN_FINISHED","threadId":"t","runId":"r"}\n\n',
          ),
        )
        .toList();

    expect(first.whereType<RunErrorEvent>(), hasLength(1));
    expect(second.whereType<RunFinishedEvent>(), hasLength(1));
  });
}
