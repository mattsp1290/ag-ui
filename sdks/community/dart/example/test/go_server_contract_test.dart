@Tags(['requires-go-server'])
library;

import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:ag_ui/ag_ui.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

import 'package:ag_ui_example/models/chat_message.dart';
import 'package:ag_ui_example/models/endpoint_config.dart';
import 'package:ag_ui_example/pages/chat_page.dart';
import 'package:ag_ui_example/pages/client_tools_page.dart';
import 'package:ag_ui_example/pages/live_state_page.dart';
import 'package:ag_ui_example/services/ag_ui_service.dart';

import 'helpers/go_server_container.dart';
import 'helpers/production_image_checks.dart';

const _requiredRoutes = {
  '/agentic_chat',
  '/human_in_the_loop',
  '/agentic_generative_ui',
  '/tool_based_generative_ui',
  '/shared_state',
  '/predictive_state_updates',
  '/image-gen',
  '/vision',
  '/audio',
  '/document',
  '/reasoning',
};

late GoServerContainer _fixture;
late Uri _baseUri;

class _RecordedRequest {
  final Uri url;
  final String body;
  final DateTime sentAt;

  _RecordedRequest(this.url, this.body) : sentAt = DateTime.now();
}

class _RecordingClient extends http.BaseClient {
  final http.Client _delegate = http.Client();
  final List<_RecordedRequest> requests = [];

  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) {
    final body = request is http.Request ? request.body : '';
    requests.add(_RecordedRequest(request.url, body));
    return _delegate.send(request);
  }

  @override
  void close() => _delegate.close();
}

void main() {
  tearDownAll(() async {
    await _fixture.close();
  });

  setUpAll(() async {
    _fixture = GoServerContainer();
    await _fixture.build(GoImage.contract);
    final server = await _fixture.start();
    _baseUri = await _fixture.baseUrl(server);
  });

  test('production image startup and scripted route checks', () async {
    await verifyProductionImages(_fixture);
  }, timeout: const Timeout(Duration(minutes: 5)));

  test(
    'health metadata exposes every Flutter route and invalid JSON is rejected',
    () async {
      final health = await http.get(_baseUri);
      expect(health.statusCode, 200);
      final metadata = jsonDecode(health.body) as Map<String, dynamic>;
      expect(metadata['provider'], 'fixture');
      expect(metadata['model'], 'content-addressed');
      expect(
        (metadata['routes'] as List).cast<String>(),
        containsAll(_requiredRoutes),
      );

      for (final route in _requiredRoutes) {
        final response = await http.post(
          _baseUri.resolve(route.substring(1)),
          headers: {
            'content-type': 'application/json',
            'accept': 'text/event-stream',
          },
          body: '{',
        );
        expect(response.statusCode, 400, reason: route);
        expect(
          response.headers['content-type'],
          startsWith('application/json'),
          reason: route,
        );
        expect(jsonDecode(response.body), {
          'error': 'invalid request body',
        }, reason: route);
      }
    },
  );

  test(
    'plain chat keeps input and streamed IDs through snapshots and a later turn',
    () async {
      final service = _service();
      addTearDown(service.close);
      final user = UserMessage(id: 'caller-user-1', content: 'fixture:plain');
      final first = await _events(
        service.run('agentic_chat', threadId: 'plain-thread', messages: [user]),
      );
      final start = first.whereType<TextMessageStartEvent>().single;
      final snapshot = first.whereType<MessagesSnapshotEvent>().single.messages;
      expect(snapshot.where((message) => message.id == user.id), hasLength(1));
      expect(
        snapshot.where((message) => message.id == start.messageId),
        hasLength(1),
      );
      final answer = snapshot.whereType<AssistantMessage>().last;
      expect(answer.content, 'Plain fixture response.');

      final later = UserMessage(
        id: 'caller-user-2',
        content: 'fixture:plain later',
      );
      final second = await _events(
        service.run(
          'agentic_chat',
          threadId: 'plain-thread',
          messages: [...snapshot, later],
        ),
      );
      final laterSnapshot = second
          .whereType<MessagesSnapshotEvent>()
          .single
          .messages;
      for (final id in [user.id, answer.id, later.id]) {
        expect(
          laterSnapshot.where((message) => message.id == id),
          hasLength(1),
        );
      }
    },
  );

  test(
    'calculate and time tool results continue to deterministic final answers',
    () async {
      await _assertToolRoundTrip(
        prompt: 'fixture:calculate',
        tool: _endpoint(
          'agentic_chat',
        ).tools.firstWhere((tool) => tool.name == 'calculate'),
        callId: 'fixture-calculate-1',
        result: '{"result":42}',
        finalText: 'The calculated result is 42.',
      );
      await _assertToolRoundTrip(
        prompt: 'fixture:time',
        tool: _endpoint(
          'agentic_chat',
        ).tools.firstWhere((tool) => tool.name == 'get_current_time'),
        callId: 'fixture-time-1',
        result: '{"time":"09:30 UTC"}',
        finalText: 'The fixture time is 09:30 UTC.',
      );
    },
  );

  test(
    'approval waits for a decision and approve/deny each continue once',
    () async {
      for (final approve in [true, false]) {
        final recorder = _RecordingClient();
        final service = _service(recorder);
        final state = ClientToolsPageState(
          endpoint: _endpoint('human_in_the_loop'),
          service: service,
        );
        addTearDown(state.dispose);
        final done = state.sendMessage('fixture:approval');
        await _waitUntil(() => state.pendingApproval != null);
        expect(state.busy, isTrue);
        expect(recorder.requests, hasLength(1));
        await Future<void>.delayed(const Duration(milliseconds: 150));
        expect(
          recorder.requests,
          hasLength(1),
          reason: 'no continuation may start before the local decision',
        );
        expect(
          state.messages.where(
            (message) => message.type == ChatMessageType.assistant,
          ),
          hasLength(1),
        );
        approve ? state.approve() : state.deny();
        await done;
        expect(recorder.requests, hasLength(2));
        final continuation = _requestJson(recorder.requests.last);
        final results = (continuation['messages'] as List)
            .cast<Map<String, dynamic>>()
            .where((message) => message['toolCallId'] == 'fixture-approval-1');
        expect(results, hasLength(1));
        expect(state.pendingApproval, isNull);
        expect(
          state.messages.any(
            (message) => message.content.contains(
              approve
                  ? 'approved and continued'
                  : 'denied and was not performed',
            ),
          ),
          isTrue,
        );
      }

      final offRecorder = _RecordingClient();
      final offService = _service(offRecorder);
      final off = ClientToolsPageState(
        endpoint: _endpoint('human_in_the_loop'),
        service: offService,
      );
      addTearDown(off.dispose);
      off.setApprovalGate(false);
      await off.sendMessage('fixture:approval');
      expect(off.pendingApproval, isNull);
      expect(
        off.messages.any(
          (message) => message.content.contains('Approval is off'),
        ),
        isTrue,
      );
      expect(offRecorder.requests, hasLength(1));
    },
  );

  test(
    'ClientToolsPageState carries tool continuation and final reply into a later turn',
    () async {
      for (final prompt in ['fixture:calculate', 'fixture:time']) {
        final recorder = _RecordingClient();
        final state = ClientToolsPageState(
          endpoint: _endpoint('agentic_chat'),
          service: _service(recorder),
        );
        addTearDown(state.dispose);
        await state.sendMessage(prompt);
        expect(recorder.requests, hasLength(2), reason: prompt);
        final expected = prompt.endsWith('calculate')
            ? 'The calculated result is 42.'
            : 'The fixture time is 09:30 UTC.';
        final finalMessage = state.messages.singleWhere(
          (message) =>
              message.type == ChatMessageType.assistant &&
              message.content == expected,
        );
        await state.sendMessage('fixture:plain');
        expect(recorder.requests, hasLength(3), reason: prompt);
        final laterMessages =
            (_requestJson(recorder.requests.last)['messages'] as List)
                .cast<Map<String, dynamic>>();
        expect(
          laterMessages.where((message) => message['id'] == finalMessage.id),
          hasLength(1),
          reason:
              'final assistant identity must be sent on the later user turn',
        );
        await state.sendMessage(prompt);
        expect(recorder.requests, hasLength(5), reason: 'repeated tool turn');
        final repeatedHistory =
            (_requestJson(recorder.requests.last)['messages'] as List)
                .cast<Map<String, dynamic>>();
        final toolId = prompt.endsWith('calculate')
            ? 'fixture-calculate-1'
            : 'fixture-time-1';
        for (final id in [toolId, '$toolId-turn-2']) {
          expect(
            repeatedHistory.where((message) => message['toolCallId'] == id),
            hasLength(1),
            reason: 'each matching user turn executes and settles once',
          );
        }
        expect(
          state.messages.where(
            (message) =>
                message.type == ChatMessageType.assistant &&
                message.content == expected,
          ),
          hasLength(2),
        );
      }
    },
  );

  test(
    'card tool renders stable facts once and reaches its continuation',
    () async {
      final recorder = _RecordingClient();
      final state = ClientToolsPageState(
        endpoint: _endpoint('tool_based_generative_ui'),
        service: _service(recorder),
      );
      addTearDown(state.dispose);
      await state.sendMessage('fixture:card');
      final cards = state.messages
          .where((message) => message.type == ChatMessageType.card)
          .toList();
      expect(cards, hasLength(1));
      expect(cards.single.cardData?['title'], 'Fixture card');
      expect(cards.single.cardData?['facts'], [
        {'label': 'Mode', 'value': 'stable'},
        {'label': 'Executions', 'value': 'one'},
      ]);
      expect(
        state.messages.any(
          (message) => message.content == 'The card was rendered once.',
        ),
        isTrue,
      );
      expect(recorder.requests, hasLength(2));
      final resultMessages = recorder.requests
          .map(_requestJson)
          .expand((request) => request['messages'] as List)
          .where(
            (message) =>
                message is Map && message['toolCallId'] == 'fixture-card-1',
          );
      expect(resultMessages, hasLength(1));
    },
  );

  test('checklist streams intermediate status before completion', () async {
    final service = _service();
    addTearDown(service.close);
    final events = await _events(
      service.run(
        'agentic_generative_ui',
        threadId: 'checklist-thread',
        messages: [UserMessage(id: 'checklist-user', content: 'fixture:plain')],
      ),
    );
    final finished = events.indexWhere((event) => event is RunFinishedEvent);
    final deltas = events.whereType<StateDeltaEvent>().toList();
    expect(deltas, isNotEmpty);
    expect(events.indexOf(deltas.first), lessThan(finished));
    expect(jsonEncode(deltas), contains('in_progress'));
    expect(jsonEncode(deltas), contains('completed'));
  });

  test(
    'shared recipe edit and predictive success/failure preserve committed state',
    () async {
      final sharedRecorder = _RecordingClient();
      final shared = LiveStatePageState(
        endpoint: _endpoint('shared_state'),
        service: _service(sharedRecorder),
      );
      addTearDown(shared.dispose);
      shared.editTitle('Client title');
      shared.changeServings(1);
      shared.addIngredient('basil', '2 leaves');
      final localRecipe =
          (shared.doc as Map<String, dynamic>)['recipe']
              as Map<String, dynamic>;
      localRecipe['clientOnly'] = 'preserve me';
      final sentState = jsonDecode(jsonEncode(shared.doc));
      await shared.sendMessage('fixture:shared-state');
      expect(_requestJson(sharedRecorder.requests.single)['state'], sentState);
      final recipe =
          (shared.doc as Map<String, dynamic>)['recipe']
              as Map<String, dynamic>;
      expect(recipe['title'], 'Fixture Soup');
      expect(recipe['servings'], 4);
      expect(recipe['steps'], contains('Serve warm.'));
      expect(recipe['clientOnly'], 'preserve me');
      expect(
        recipe['ingredients'],
        contains(equals({'name': 'basil', 'amount': '2 leaves'})),
      );

      final predictiveRecorder = _RecordingClient();
      final predictive = LiveStatePageState(
        endpoint: _endpoint('predictive_state_updates'),
        service: _service(predictiveRecorder),
      );
      addTearDown(predictive.dispose);
      var observedDraft = false;
      predictive.addListener(() {
        final value = predictive.doc;
        if (value is Map && value['_predictive'] is Map) {
          observedDraft = true;
        }
      });
      await predictive.sendMessage('fixture:predictive');
      expect(observedDraft, isTrue);
      expect((predictive.doc as Map).containsKey('_predictive'), isFalse);
      expect(((predictive.doc as Map)['recipe'] as Map)['steps'], [
        'Prepare ingredients.',
        'Serve warm.',
      ]);
      observedDraft = false;
      await predictive.sendMessage('fixture:predictive-failure');
      expect((predictive.doc as Map).containsKey('_predictive'), isFalse);
      expect(((predictive.doc as Map)['recipe'] as Map)['steps'], [
        'Prepare ingredients.',
        'Serve warm.',
      ]);
      expect(predictive.lastSummary, contains('Run error'));
      expect(
        observedDraft,
        isTrue,
        reason: 'failure must stream a partial draft',
      );
      await predictive.sendMessage('fixture:predictive');
      final stateAfterFailure =
          _requestJson(predictiveRecorder.requests.last)['state']
              as Map<String, dynamic>;
      expect(stateAfterFailure.containsKey('_predictive'), isFalse);
    },
  );

  test(
    'reasoning events are balanced and reconcile with one snapshot item',
    () async {
      final service = _service();
      addTearDown(service.close);
      final events = await _events(
        service.run(
          'reasoning',
          threadId: 'reasoning-thread',
          messages: [
            UserMessage(id: 'reasoning-user', content: 'show reasoning'),
          ],
        ),
      );
      expect(events.whereType<ReasoningStartEvent>(), hasLength(1));
      expect(events.whereType<ReasoningMessageStartEvent>(), hasLength(1));
      expect(events.whereType<ReasoningMessageEndEvent>(), hasLength(1));
      expect(events.whereType<ReasoningEndEvent>(), hasLength(1));
      final reasoningId = events
          .whereType<ReasoningMessageStartEvent>()
          .single
          .messageId;
      final snapshot = events
          .whereType<MessagesSnapshotEvent>()
          .single
          .messages;
      expect(
        snapshot.whereType<ReasoningMessage>().where(
          (message) => message.id == reasoningId,
        ),
        hasLength(1),
      );
      expect(snapshot.whereType<AssistantMessage>(), hasLength(1));
    },
  );

  test(
    'media success and provider errors cross the production handlers',
    () async {
      final bytes = base64Encode(Uint8List.fromList([1, 2, 3]));
      final cases = <(String, List<InputContent>, String)>[
        (
          'vision',
          [
            ImageInputContent(
              source: DataSource(value: bytes, mimeType: 'image/png'),
            ),
            const TextInputContent('fixture:plain'),
          ],
          'Fixture image',
        ),
        (
          'audio',
          [
            AudioInputContent(
              source: DataSource(value: bytes, mimeType: 'audio/wav'),
            ),
          ],
          'Fixture audio transcription',
        ),
        (
          'document',
          [
            DocumentInputContent(
              source: DataSource(value: bytes, mimeType: 'application/pdf'),
            ),
            const TextInputContent('fixture:plain'),
          ],
          'Fixture document summary',
        ),
      ];
      for (final item in cases) {
        final service = _service();
        addTearDown(service.close);
        final events = await _events(
          service.sendMultimodalMessage(item.$1, item.$2),
        );
        expect(_snapshotText(events), contains(item.$3));
        await service.close();
      }

      final imageService = _service();
      addTearDown(imageService.close);
      final imageEvents = await _events(
        imageService.sendMessage('image-gen', 'fixture:plain'),
      );
      final image = imageEvents.whereType<CustomEvent>().singleWhere(
        (event) => event.name == 'image_generated',
      );
      expect((image.value as Map)['url'], startsWith('data:image/png;base64,'));
      await imageService.close();

      for (final route in ['image-gen', 'vision', 'audio', 'document']) {
        final service = _service();
        addTearDown(service.close);
        final events = route == 'image-gen'
            ? await _events(
                service.sendMessage(route, 'fixture:provider-failure'),
              )
            : await _events(
                service.sendMultimodalMessage(route, [
                  if (route == 'vision')
                    ImageInputContent(
                      source: DataSource(value: bytes, mimeType: 'image/png'),
                    ),
                  if (route == 'document')
                    DocumentInputContent(
                      source: DataSource(
                        value: bytes,
                        mimeType: 'application/pdf',
                      ),
                    ),
                  if (route == 'audio')
                    AudioInputContent(
                      source: DataSource(
                        value: 'Zml4dHVyZS1wcm92aWRlci1mYWlsdXJl',
                        mimeType: 'audio/wav',
                      ),
                    ),
                  if (route != 'audio')
                    const TextInputContent('fixture:provider-failure'),
                ]),
              );
        expect(events.whereType<RunErrorEvent>(), hasLength(1), reason: route);
        expect(events.last, isA<RunErrorEvent>(), reason: route);
        expect(events.whereType<RunFinishedEvent>(), isEmpty, reason: route);
      }
    },
  );

  test('invalid media input becomes a visible page error', () async {
    for (final route in ['vision', 'audio', 'document']) {
      final state = ChatPageState(
        endpoint: _endpoint(route),
        service: _service(),
      );
      addTearDown(state.dispose);
      state.sendMessage('text without media');
      await _waitUntil(() => !state.isLoading);
      expect(
        state.messages.where(
          (message) =>
              message.type == ChatMessageType.system &&
              message.content.contains(
                'no ${route == 'vision' ? 'image' : route} part',
              ),
        ),
        isNotEmpty,
        reason: route,
      );
    }

    // The image-gen page rejects blank input before transport. Exercise its real
    // HTTP 400 boundary through the service rather than bypassing page validation.
    final imageService = _service();
    addTearDown(imageService.close);
    await expectLater(
      _events(
        imageService.run(
          'image-gen',
          threadId: 'invalid-image',
          messages: const [],
        ),
      ),
      throwsA(isA<AGUIError>()),
    );

    // A failed media request must release the service for a valid retry.
    final retryBytes = base64Encode(Uint8List.fromList([1, 2, 3]));
    for (final route in ['audio', 'document']) {
      final service = _service();
      addTearDown(service.close);
      final missing = await _events(
        service.run(route, threadId: 'missing-$route', messages: const []),
      );
      expect(missing.whereType<RunErrorEvent>(), hasLength(1));
      final parts = route == 'audio'
          ? <InputContent>[
              AudioInputContent(
                source: DataSource(value: retryBytes, mimeType: 'audio/wav'),
              ),
            ]
          : <InputContent>[
              DocumentInputContent(
                source: DataSource(
                  value: retryBytes,
                  mimeType: 'application/pdf',
                ),
              ),
              const TextInputContent('fixture:plain'),
            ];
      final retry = await _events(service.sendMultimodalMessage(route, parts));
      expect(retry.whereType<RunFinishedEvent>(), hasLength(1));
    }
  });

  test(
    'reasoning reconciles to one reasoning and one answer in ChatPageState',
    () async {
      final state = ChatPageState(
        endpoint: _endpoint('reasoning'),
        service: _service(),
      );
      addTearDown(state.dispose);
      state.sendMessage('show scripted reasoning');
      await _waitUntil(() => state.isLoading);
      await _waitUntil(() => !state.isLoading);
      expect(
        state.messages.where(
          (message) => message.type == ChatMessageType.reasoning,
        ),
        hasLength(1),
      );
      expect(
        state.messages.where(
          (message) =>
              message.type == ChatMessageType.assistant &&
              message.content.contains('scripted reasoning demonstration'),
        ),
        hasLength(1),
      );
    },
  );

  test(
    'dispose during delayed response produces no late notifications or continuation',
    () async {
      final recorder = _RecordingClient();
      final state = ClientToolsPageState(
        endpoint: _endpoint('agentic_chat'),
        service: _service(recorder),
      );
      addTearDown(state.dispose);
      var notifications = 0;
      state.addListener(() => notifications++);
      final pending = state.sendMessage('fixture:delayed');
      await Future<void>.delayed(const Duration(milliseconds: 100));
      expect(recorder.requests, hasLength(1));
      state.dispose();
      final count = notifications;
      await pending;
      await Future<void>.delayed(const Duration(milliseconds: 1000));
      expect(notifications, count);
      expect(
        state.messages.where(
          (message) => message.content.contains('Delayed fixture response'),
        ),
        isEmpty,
      );
      expect(
        recorder.requests,
        hasLength(1),
        reason: 'disposing must not launch a continuation request',
      );
    },
  );

  test(
    'interrupted model stream is visible and never reported as success',
    () async {
      final recorder = _RecordingClient();
      final state = ChatPageState(
        endpoint: _endpoint('agentic_chat'),
        service: _service(recorder),
      );
      addTearDown(state.dispose);
      state.sendMessage('fixture:interrupted');
      await _waitUntil(() => !state.isLoading);
      expect(recorder.requests, hasLength(1));
      expect(
        state.messages.any(
          (message) =>
              message.type == ChatMessageType.system &&
              message.content.toLowerCase().contains('error'),
        ),
        isTrue,
      );
      expect(
        state.messages.any(
          (message) =>
              message.content.contains('Deterministic fixture response'),
        ),
        isFalse,
      );
    },
  );
}

AgUiService _service([http.Client? client]) =>
    AgUiService(baseUrl: _baseUri.toString(), httpClient: client);

Map<String, dynamic> _requestJson(_RecordedRequest request) =>
    jsonDecode(request.body) as Map<String, dynamic>;

EndpointConfig _endpoint(String path) =>
    EndpointConfig.availableEndpoints.singleWhere((item) => item.path == path);

Future<List<BaseEvent>> _events(Stream<BaseEvent> stream) =>
    stream.toList().timeout(const Duration(seconds: 15));

String _snapshotText(List<BaseEvent> events) => events
    .whereType<MessagesSnapshotEvent>()
    .expand((event) => event.messages)
    .whereType<AssistantMessage>()
    .map((message) => message.content ?? '')
    .join('\n');

Future<void> _waitUntil(bool Function() condition) async {
  final deadline = DateTime.now().add(const Duration(seconds: 10));
  while (!condition()) {
    if (DateTime.now().isAfter(deadline)) {
      throw TimeoutException('condition not reached');
    }
    await Future<void>.delayed(const Duration(milliseconds: 20));
  }
}

Future<void> _assertToolRoundTrip({
  required String prompt,
  required Tool tool,
  required String callId,
  required String result,
  required String finalText,
}) async {
  final service = _service();
  addTearDown(service.close);
  final user = UserMessage(id: 'user-$callId', content: prompt);
  final first = await _events(
    service.run(
      'agentic_chat',
      threadId: 'thread-$callId',
      messages: [user],
      tools: [tool],
    ),
  );
  final firstSnapshot = first
      .whereType<MessagesSnapshotEvent>()
      .single
      .messages;
  final owner = firstSnapshot.whereType<AssistantMessage>().singleWhere(
    (message) => message.toolCalls?.any((call) => call.id == callId) ?? false,
  );
  final toolResult = ToolMessage(
    id: 'result-$callId',
    toolCallId: callId,
    content: result,
  );
  final incorrect = await _events(
    service.run(
      'agentic_chat',
      threadId: 'thread-$callId-wrong-result',
      messages: [
        ...firstSnapshot,
        ToolMessage(
          id: 'wrong-result-$callId',
          toolCallId: 'wrong-$callId',
          content: result,
        ),
      ],
      tools: [tool],
    ),
  );
  expect(
    incorrect.whereType<ToolCallStartEvent>().any(
      (event) => event.toolCallId == callId,
    ),
    isTrue,
    reason: 'a mismatched tool result must not unlock the final answer',
  );
  expect(
    incorrect.whereType<TextMessageContentEvent>().any(
      (event) => event.delta.contains(finalText),
    ),
    isFalse,
  );
  final second = await _events(
    service.run(
      'agentic_chat',
      threadId: 'thread-$callId',
      messages: [...firstSnapshot, toolResult],
      tools: [tool],
    ),
  );
  final secondSnapshot = second
      .whereType<MessagesSnapshotEvent>()
      .single
      .messages;
  expect(
    secondSnapshot.where((message) => message.id == user.id),
    hasLength(1),
  );
  expect(
    secondSnapshot.where((message) => message.id == owner.id),
    hasLength(1),
  );
  expect(
    secondSnapshot.where((message) => message.id == toolResult.id),
    hasLength(1),
  );
  expect(secondSnapshot.whereType<AssistantMessage>().last.content, finalText);
  await service.close();
}
