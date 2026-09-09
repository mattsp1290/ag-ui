import 'dart:convert';

import 'package:ag_ui/ag_ui.dart';
import 'package:test/test.dart';

void _expectCipherSafe(AGUIValidationError error, String secret) {
  expect(error.message, isNot(contains(secret)));
  expect(error.value?.toString(), isNot(contains(secret)));
  expect(error.json?.toString(), isNot(contains(secret)));
  expect(error.cause?.toString(), isNot(contains(secret)));
}

void main() {
  group('subagent lifecycle events', () {
    test('started round-trips every optional link through public API', () {
      const event = SubagentStartedEvent(
        subagentRunId: 'child-1',
        name: 'researcher',
        description: 'Find evidence',
        parentSubagentRunId: 'parent-1',
        parentToolCallId: 'call-1',
        parentMessageId: 'message-1',
        timestamp: 1700000001,
        metadata: {'source': 'test'},
        rawEvent: {'source': 'wire'},
      );

      final decoded = BaseEvent.fromJson(event.toJson());
      expect(decoded, isA<SubagentStartedEvent>());
      expect(decoded.toJson(), event.toJson());
    });

    test('started reads aliases with camelCase presence precedence', () {
      final snake = SubagentStartedEvent.fromJson({
        'type': 'SUBAGENT_STARTED',
        'subagent_run_id': 'snake-child',
        'name': 'worker',
        'parent_subagent_run_id': 'snake-parent',
        'parent_tool_call_id': 'snake-tool',
        'parent_message_id': 'snake-message',
        'raw_event': {'source': 'snake'},
      });
      expect(snake.subagentRunId, 'snake-child');
      expect(snake.parentSubagentRunId, 'snake-parent');
      expect(snake.parentToolCallId, 'snake-tool');
      expect(snake.parentMessageId, 'snake-message');

      final camel = SubagentStartedEvent.fromJson({
        'type': 'SUBAGENT_STARTED',
        'subagentRunId': 'camel-child',
        'subagent_run_id': 'snake-child',
        'name': 'worker',
        'parentSubagentRunId': null,
        'parent_subagent_run_id': 'snake-parent',
      });
      expect(camel.subagentRunId, 'camel-child');
      expect(camel.parentSubagentRunId, isNull);
    });

    test('started copy preserves omissions and clears every optional field',
        () {
      const original = SubagentStartedEvent(
        subagentRunId: 'child',
        name: 'worker',
        description: 'description',
        parentSubagentRunId: 'parent',
        parentToolCallId: 'tool',
        parentMessageId: 'message',
        metadata: {'key': 'value'},
      );

      expect(original.copyWith().toJson(), original.toJson());
      expect(
        original
            .copyWith(
              description: null,
              parentSubagentRunId: null,
              parentToolCallId: null,
              parentMessageId: null,
              metadata: null,
            )
            .toJson(),
        {
          'type': 'SUBAGENT_STARTED',
          'subagentRunId': 'child',
          'name': 'worker',
        },
      );
    });

    test('finished preserves result shapes beside typed outcome', () {
      final values = <Object>[
        false,
        0,
        'done',
        <Object?>[1, null, false],
        <String, Object?>{
          'nested': [null, 1],
        },
      ];
      for (final value in values) {
        final event = SubagentFinishedEvent(
          subagentRunId: 'child',
          result: value,
          outcome: const SubagentFinishedSuspendedOutcome(
            interruptIds: ['interrupt-1'],
          ),
        );
        expect(
          SubagentFinishedEvent.fromJson(event.toJson()).toJson(),
          event.toJson(),
          reason: value.toString(),
        );
      }
    });

    test('finished normalizes absent and null result/outcome to omission', () {
      for (final json in <Map<String, dynamic>>[
        {'type': 'SUBAGENT_FINISHED', 'subagentRunId': 'child'},
        {
          'type': 'SUBAGENT_FINISHED',
          'subagentRunId': 'child',
          'result': null,
          'outcome': null,
        },
      ]) {
        final event = SubagentFinishedEvent.fromJson(json);
        expect(event.result, isNull);
        expect(event.outcome, isNull);
        expect(event.toJson().containsKey('result'), isFalse);
        expect(event.toJson().containsKey('outcome'), isFalse);
      }
    });

    test('finished copy can clear result, outcome, and metadata', () {
      const event = SubagentFinishedEvent(
        subagentRunId: 'child',
        result: {'ok': true},
        outcome: SubagentFinishedSuccessOutcome(),
        metadata: {'source': 'test'},
      );

      expect(event.copyWith().toJson(), event.toJson());
      expect(
        event.copyWith(result: null, outcome: null, metadata: null).toJson(),
        {'type': 'SUBAGENT_FINISHED', 'subagentRunId': 'child'},
      );
    });

    test('error round-trips optional code and copy can clear it', () {
      const event = SubagentErrorEvent(
        subagentRunId: 'child',
        message: 'failed',
        code: 'E_TOOL',
        metadata: {'source': 'test'},
      );
      expect(BaseEvent.fromJson(event.toJson()).toJson(), event.toJson());
      expect(
        event.copyWith(code: null, metadata: null).toJson(),
        {
          'type': 'SUBAGENT_ERROR',
          'subagentRunId': 'child',
          'message': 'failed',
        },
      );
    });

    test('direct JSON and SSE codecs preserve all three event classes', () {
      const events = <BaseEvent>[
        SubagentStartedEvent(subagentRunId: 'child', name: 'worker'),
        SubagentFinishedEvent(
          subagentRunId: 'child',
          outcome: SubagentFinishedSuccessOutcome(),
        ),
        SubagentErrorEvent(subagentRunId: 'other', message: 'failed'),
      ];
      final encoder = EventEncoder();
      const decoder = EventDecoder();

      for (final event in events) {
        expect(
          decoder.decode(jsonEncode(event.toJson())).runtimeType,
          event.runtimeType,
        );
        expect(
          decoder.decodeSSE(encoder.encodeSSE(event)).toJson(),
          event.toJson(),
        );
      }
    });

    test('malformed IDs, outcome, and cipher-bearing errors are safe', () {
      for (final json in <Map<String, dynamic>>[
        {'type': 'SUBAGENT_STARTED', 'name': 'worker'},
        {
          'type': 'SUBAGENT_FINISHED',
          'subagentRunId': 1,
        },
        {
          'type': 'SUBAGENT_FINISHED',
          'subagentRunId': 'child',
          'outcome': {'type': 'unknown'},
        },
        {
          'type': 'SUBAGENT_ERROR',
          'subagentRunId': 'child',
        },
      ]) {
        expect(
          () => BaseEvent.fromJson(json),
          throwsA(isA<AGUIValidationError>()),
          reason: json.toString(),
        );
      }

      const secret = 'cipher-secret-subagent-event';
      for (final json in <Map<String, dynamic>>[
        {
          'type': 'SUBAGENT_STARTED',
          'subagentRunId': 1,
          'name': 'worker',
          'metadata': {'encryptedValue': secret},
        },
        {
          'type': 'SUBAGENT_FINISHED',
          'subagentRunId': 'child',
          'result': {'encryptedValue': secret},
          'outcome': {'type': 'unknown', 'detail': secret},
        },
        {
          'type': 'SUBAGENT_ERROR',
          'subagentRunId': 'child',
          'message': 1,
          'rawEvent': {'encryptedValue': secret},
        },
      ]) {
        try {
          BaseEvent.fromJson(json);
          fail('Expected validation error');
        } on AGUIValidationError catch (error) {
          _expectCipherSafe(error, secret);
        }
      }
    });

    test('stream lifecycle keeps child terminals observable and reuses IDs',
        () async {
      final frames = <Map<String, dynamic>>[
        {'type': 'RUN_STARTED', 'threadId': 't', 'runId': 'r1'},
        {
          'type': 'SUBAGENT_STARTED',
          'subagentRunId': 'parent',
          'name': 'parent',
        },
        {
          'type': 'SUBAGENT_STARTED',
          'subagentRunId': 'child',
          'name': 'child',
          'parentSubagentRunId': 'parent',
        },
        {
          'type': 'TEXT_MESSAGE_CHUNK',
          'messageId': 'message',
          'delta': 'working',
          'subagentRunId': 'child',
        },
        {
          'type': 'SUBAGENT_FINISHED',
          'subagentRunId': 'child',
          'result': {'partial': true},
          'outcome': {
            'type': 'suspended',
            'interruptIds': ['interrupt-1'],
          },
        },
        {
          'type': 'SUBAGENT_FINISHED',
          'subagentRunId': 'parent',
          'outcome': {'type': 'suspended'},
        },
        {
          'type': 'RUN_FINISHED',
          'threadId': 't',
          'runId': 'r1',
          'outcome': {
            'type': 'interrupt',
            'interrupts': [
              {
                'id': 'interrupt-1',
                'reason': 'approval',
                'subagentRunId': 'child',
              },
            ],
          },
        },
        {'type': 'RUN_STARTED', 'threadId': 't', 'runId': 'r2'},
        {
          'type': 'SUBAGENT_STARTED',
          'subagentRunId': 'child',
          'name': 'child',
        },
        {
          'type': 'SUBAGENT_ERROR',
          'subagentRunId': 'sibling',
          'message': 'failed independently',
        },
        {
          'type': 'SUBAGENT_FINISHED',
          'subagentRunId': 'child',
          'result': false,
          'outcome': {'type': 'success'},
        },
        {'type': 'RUN_FINISHED', 'threadId': 't', 'runId': 'r2'},
      ];
      final stream = Stream<String>.fromIterable(
        frames.map((json) => 'data: ${jsonEncode(json)}\n\n'),
      );

      final events =
          await EventStreamAdapter().fromRawSseStream(stream).toList();
      expect(events, hasLength(frames.length));
      expect(events.whereType<SubagentStartedEvent>(), hasLength(3));
      expect(
        events.whereType<SubagentStartedEvent>().last.subagentRunId,
        'child',
      );
      expect(events.whereType<SubagentErrorEvent>(), hasLength(1));
      expect(events.last, isA<RunFinishedEvent>());
    });
  });
}
