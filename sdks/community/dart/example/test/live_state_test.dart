import 'dart:async';
import 'dart:convert';
import 'package:ag_ui/ag_ui.dart';
import 'package:ag_ui_example/models/endpoint_config.dart';
import 'package:ag_ui_example/pages/live_state_page.dart';
import 'package:ag_ui_example/services/ag_ui_service.dart';
import 'package:flutter_test/flutter_test.dart';

class StateService extends AgUiService {
  final List<Stream<BaseEvent>> runs;
  final List<Object?> terminalErrors;
  final states = <dynamic>[];
  final histories = <List<Message>>[];
  final threads = <String>[];
  StateService(this.runs, {this.terminalErrors = const []});

  @override
  Stream<BaseEvent> run(
    String endpoint, {
    required String threadId,
    required List<Message> messages,
    List<Tool> tools = const [],
    dynamic state,
    Map<String, String> extraQuery = const {},
  }) {
    states.add(jsonDecode(jsonEncode(state)));
    histories.add(
      messages.map((message) => Message.fromJson(message.toJson())).toList(),
    );
    threads.add(threadId);
    final index = states.length - 1;
    final source = runs[index];
    final error = index < terminalErrors.length ? terminalErrors[index] : null;
    if (error == null) return source;
    return (() async* {
      await for (final event in source) {
        yield event;
      }
      throw error;
    })();
  }
}

final finished = RunFinishedEvent(threadId: 'thread', runId: 'run');
EndpointConfig endpoint(String path) =>
    EndpointConfig.availableEndpoints.singleWhere((e) => e.path == path);
StateDeltaEvent patch(List<Map<String, dynamic>> delta) =>
    StateDeltaEvent(delta: delta);

void main() {
  test(
    'idle edits are sent; active edits are ignored and document snapshots are independent',
    () async {
      final stream = StreamController<BaseEvent>();
      final service = StateService([stream.stream]);
      final state = LiveStatePageState(
        endpoint: endpoint('shared_state'),
        service: service,
      );
      addTearDown(state.dispose);
      state.editTitle('Local soup');
      state.changeServings(1);
      final exchange = state.sendMessage('add basil');
      await pumpEventQueue();
      expect(service.states.single['recipe']['title'], 'Local soup');
      expect(service.states.single['recipe']['servings'], 3);
      state.editTitle('should be ignored');
      state.changeServings(10);
      state.addIngredient('ignored', '1');
      state.removeIngredient(0);
      expect(state.doc['recipe']['title'], 'Local soup');
      expect(state.doc['recipe']['servings'], 3);
      expect(state.doc['recipe']['ingredients'], hasLength(2));
      stream.add(
        patch([
          {'op': 'replace', 'path': '/recipe/title', 'value': 'Updated soup'},
        ]),
      );
      await pumpEventQueue();
      expect(state.doc['recipe']['title'], 'Updated soup');
      expect(service.states.single['recipe']['title'], 'Local soup');
      stream.add(finished);
      await exchange;
      await stream.close();
      expect(state.busy, isFalse);
    },
  );

  test(
    'a failed delta batch leaves the last valid document intact and ignores trailing events',
    () async {
      final service = StateService([
        Stream.fromIterable([
          patch([
            {
              'op': 'replace',
              'path': '/recipe/title',
              'value': 'Partial corruption',
            },
            {'op': 'replace', 'path': '/missing/child', 'value': 1},
          ]),
          patch([
            {
              'op': 'replace',
              'path': '/recipe/title',
              'value': 'Trailing corruption',
            },
          ]),
          finished,
        ]),
      ]);
      final state = LiveStatePageState(
        endpoint: endpoint('shared_state'),
        service: service,
      );
      addTearDown(state.dispose);
      await state.sendMessage('edit');
      expect(state.doc['recipe']['title'], 'Tomato Pasta');
      expect(state.lastSummary, contains('Error:'));
      expect(state.busy, isFalse);
    },
  );

  for (final ending in ['success', 'provider-error', 'disconnect']) {
    test(
      'prediction cleans up on $ending while committed steps remain',
      () async {
        final events = <BaseEvent>[
          patch([
            {
              'op': 'add',
              'path': '/_predictive',
              'value': {'draft': 'Temporary draft'},
            },
          ]),
          patch([
            {
              'op': 'replace',
              'path': '/recipe/steps',
              'value': ['Committed step'],
            },
          ]),
          if (ending == 'success') finished,
          if (ending == 'provider-error')
            RunErrorEvent(message: 'provider failed'),
          if (ending == 'provider-error')
            patch([
              {
                'op': 'replace',
                'path': '/recipe/steps',
                'value': ['Must not apply'],
              },
            ]),
        ];
        final service = StateService([
          Stream.fromIterable(events),
          Stream.value(finished),
        ]);
        final state = LiveStatePageState(
          endpoint: endpoint('predictive_state_updates'),
          service: service,
        );
        addTearDown(state.dispose);
        final visibleDrafts = <String>[];
        state.addListener(() {
          final draft = (state.doc['_predictive'] as Map?)?['draft'];
          if (draft is String) visibleDrafts.add(draft);
        });
        await state.sendMessage('make steps');
        expect(visibleDrafts, contains('Temporary draft'));
        expect(state.doc.containsKey('_predictive'), isFalse);
        expect(state.doc['recipe']['steps'], ['Committed step']);
        expect(state.busy, isFalse);
        if (ending == 'provider-error') {
          expect(state.lastSummary, contains('provider failed'));
        }
        if (ending == 'disconnect') {
          expect(state.lastSummary, contains('interrupted'));
        }
        await state.sendMessage('next request');
        expect(
          (service.states.last as Map).containsKey('_predictive'),
          isFalse,
        );
      },
    );
  }

  test(
    'disposal clears prediction and prevents late stream notifications',
    () async {
      final stream = StreamController<BaseEvent>();
      final service = StateService([stream.stream]);
      final state = LiveStatePageState(
        endpoint: endpoint('predictive_state_updates'),
        service: service,
      );
      var notifications = 0;
      state.addListener(() => notifications++);
      final exchange = state.sendMessage('stream');
      stream.add(
        patch([
          {
            'op': 'add',
            'path': '/_predictive',
            'value': {'draft': 'Temporary'},
          },
        ]),
      );
      await pumpEventQueue();
      final before = notifications;
      state.dispose();
      expect(state.doc.containsKey('_predictive'), isFalse);
      stream.add(TextMessageContentEvent(messageId: 'late', delta: 'late'));
      await stream.close();
      await exchange;
      expect(notifications, before);
      expect(() => state.dispose(), returnsNormally);
    },
  );

  test('malformed snapshot preserves the last valid document', () async {
    final service = StateService([
      Stream.fromIterable([
        const StateSnapshotEvent(
          snapshot: {
            'recipe': {
              'title': 42,
              'servings': 2,
              'ingredients': <dynamic>[],
              'steps': <dynamic>[],
            },
          },
        ),
        patch([
          {'op': 'replace', 'path': '/recipe/title', 'value': 'must not apply'},
        ]),
        finished,
      ]),
    ]);
    final state = LiveStatePageState(
      endpoint: endpoint('shared_state'),
      service: service,
    );
    addTearDown(state.dispose);

    await state.sendMessage('bad snapshot');

    expect(state.doc['recipe']['title'], 'Tomato Pasta');
    expect(state.lastSummary, contains('Recipe title must be a string'));
    expect(state.busy, isFalse);
  });

  test('schema-invalid delta values are atomic', () async {
    final service = StateService([
      Stream.fromIterable([
        patch([
          {'op': 'replace', 'path': '/recipe/title', 'value': 42},
        ]),
        finished,
      ]),
    ]);
    final state = LiveStatePageState(
      endpoint: endpoint('shared_state'),
      service: service,
    );
    addTearDown(state.dispose);

    await state.sendMessage('bad value');

    expect(state.doc['recipe']['title'], 'Tomato Pasta');
    expect(state.lastSummary, contains('Recipe title must be a string'));
    expect(state.busy, isFalse);
  });

  test(
    'state requests retain one thread and cumulative message history',
    () async {
      final service = StateService([
        Stream.fromIterable([
          MessagesSnapshotEvent(
            messages: const [
              AssistantMessage(id: 'assistant-1', content: 'updated'),
            ],
          ),
          finished,
        ]),
        Stream.value(finished),
      ]);
      final state = LiveStatePageState(
        endpoint: endpoint('shared_state'),
        service: service,
      );
      addTearDown(state.dispose);

      await state.sendMessage('first edit');
      await state.sendMessage('second edit');

      expect(service.threads.toSet(), hasLength(1));
      final secondHistory = service.histories[1];
      expect(secondHistory.whereType<UserMessage>(), hasLength(2));
      expect(
        secondHistory.whereType<AssistantMessage>().where(
          (message) => message.id == 'assistant-1',
        ),
        hasLength(1),
      );
    },
  );

  test('thrown errors clear predictions and permit a clean retry', () async {
    final service = StateService(
      [
        Stream.fromIterable([
          patch([
            {
              'op': 'add',
              'path': '/_predictive',
              'value': {'draft': 'temporary'},
            },
          ]),
          patch([
            {
              'op': 'replace',
              'path': '/recipe/steps',
              'value': ['committed'],
            },
          ]),
        ]),
        Stream.value(finished),
      ],
      terminalErrors: [StateError('decoder failed')],
    );
    final state = LiveStatePageState(
      endpoint: endpoint('predictive_state_updates'),
      service: service,
    );
    addTearDown(state.dispose);

    await state.sendMessage('predict');
    expect(state.doc.containsKey('_predictive'), isFalse);
    expect(state.doc['recipe']['steps'], ['committed']);
    expect(state.lastSummary, contains('decoder failed'));
    expect(state.busy, isFalse);

    await state.sendMessage('retry');
    expect(service.states, hasLength(2));
    expect((service.states.last as Map).containsKey('_predictive'), isFalse);
    expect(state.busy, isFalse);
  });
}
