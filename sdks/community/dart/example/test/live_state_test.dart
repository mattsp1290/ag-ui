import 'dart:async';
import 'dart:convert';
import 'package:ag_ui/ag_ui.dart';
import 'package:ag_ui_example/models/endpoint_config.dart';
import 'package:ag_ui_example/pages/live_state_page.dart';
import 'package:ag_ui_example/services/ag_ui_service.dart';
import 'package:flutter_test/flutter_test.dart';

class StateService extends AgUiService {
  final List<Stream<BaseEvent>> runs;
  final states = <dynamic>[];
  StateService(this.runs);

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
    return runs[states.length - 1];
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
}
