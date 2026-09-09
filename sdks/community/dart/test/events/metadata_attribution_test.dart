import 'package:ag_ui/ag_ui.dart';
import 'package:test/test.dart';

Map<String, dynamic> _payload(String type, {bool attributed = false}) {
  final payload = <String, dynamic>{
    'type': type,
    'metadata': <String, dynamic>{'event': type, 'nullValue': null},
  };
  if (attributed) {
    payload['subagent_run_id'] = 'child-$type';
  }

  switch (type) {
    case 'TEXT_MESSAGE_START':
      payload.addAll({'messageId': 'message', 'role': 'assistant'});
    case 'TEXT_MESSAGE_CONTENT':
      payload.addAll({'messageId': 'message', 'delta': 'text'});
    case 'TEXT_MESSAGE_END':
      payload['messageId'] = 'message';
    case 'TEXT_MESSAGE_CHUNK':
      payload['messageId'] = 'message';
    case 'THINKING_CONTENT':
    case 'THINKING_TEXT_MESSAGE_CONTENT':
      payload['delta'] = 'thinking';
    case 'TOOL_CALL_START':
      payload.addAll({'toolCallId': 'call', 'toolCallName': 'lookup'});
    case 'TOOL_CALL_ARGS':
      payload.addAll({'toolCallId': 'call', 'delta': '{}'});
    case 'TOOL_CALL_END':
      payload['toolCallId'] = 'call';
    case 'TOOL_CALL_CHUNK':
      payload['toolCallId'] = 'call';
    case 'TOOL_CALL_RESULT':
      payload.addAll({
        'messageId': 'message',
        'toolCallId': 'call',
        'content': 'result',
      });
    case 'STATE_SNAPSHOT':
      payload['snapshot'] = <String, dynamic>{'state': true};
    case 'STATE_DELTA':
      payload['delta'] = <dynamic>[];
    case 'MESSAGES_SNAPSHOT':
      payload['messages'] = <Map<String, dynamic>>[];
    case 'ACTIVITY_SNAPSHOT':
      payload.addAll({
        'messageId': 'activity',
        'activityType': 'PLAN',
        'content': <String, dynamic>{},
      });
    case 'ACTIVITY_DELTA':
      payload.addAll({
        'messageId': 'activity',
        'activityType': 'PLAN',
        'patch': <dynamic>[],
      });
    case 'RAW':
      payload['event'] = <String, dynamic>{'provider': 'test'};
    case 'CUSTOM':
      payload.addAll({'name': 'custom', 'value': null});
    case 'RUN_STARTED':
      payload.addAll({'threadId': 'thread', 'runId': 'run'});
    case 'RUN_FINISHED':
      payload.addAll({'threadId': 'thread', 'runId': 'run'});
    case 'RUN_ERROR':
      payload['message'] = 'failed';
    case 'STEP_STARTED':
    case 'STEP_FINISHED':
      payload['stepName'] = 'step';
    case 'REASONING_START':
    case 'REASONING_MESSAGE_START':
    case 'REASONING_MESSAGE_CONTENT':
    case 'REASONING_MESSAGE_END':
      payload['messageId'] = 'reasoning';
      if (type == 'REASONING_MESSAGE_START') {
        payload['role'] = 'reasoning';
      }
      if (type == 'REASONING_MESSAGE_CONTENT') {
        payload['delta'] = 'thinking';
      }
    case 'REASONING_MESSAGE_CHUNK':
      payload['messageId'] = 'reasoning';
    case 'REASONING_END':
      payload['messageId'] = 'reasoning';
      break;
    case 'REASONING_ENCRYPTED_VALUE':
      payload.addAll({
        'subtype': 'message',
        'entityId': 'reasoning',
        'encryptedValue': 'cipher',
      });
    case 'THINKING_START':
    case 'THINKING_END':
    case 'THINKING_TEXT_MESSAGE_START':
    case 'THINKING_TEXT_MESSAGE_END':
      break;
  }
  return payload;
}

void main() {
  const attributedTypes = <String>{
    'TEXT_MESSAGE_START',
    'TEXT_MESSAGE_CONTENT',
    'TEXT_MESSAGE_END',
    'TEXT_MESSAGE_CHUNK',
    'TOOL_CALL_START',
    'TOOL_CALL_ARGS',
    'TOOL_CALL_END',
    'TOOL_CALL_CHUNK',
    'TOOL_CALL_RESULT',
    'STATE_SNAPSHOT',
    'STATE_DELTA',
    'ACTIVITY_SNAPSHOT',
    'ACTIVITY_DELTA',
    'RAW',
    'CUSTOM',
    'STEP_STARTED',
    'STEP_FINISHED',
    'REASONING_START',
    'REASONING_MESSAGE_START',
    'REASONING_MESSAGE_CONTENT',
    'REASONING_MESSAGE_END',
    'REASONING_MESSAGE_CHUNK',
    'REASONING_END',
    'REASONING_ENCRYPTED_VALUE',
  };
  const allTypes = <String>{
    ...attributedTypes,
    'THINKING_START',
    'THINKING_CONTENT',
    'THINKING_END',
    'THINKING_TEXT_MESSAGE_START',
    'THINKING_TEXT_MESSAGE_CONTENT',
    'THINKING_TEXT_MESSAGE_END',
    'MESSAGES_SNAPSHOT',
    'RUN_STARTED',
    'RUN_FINISHED',
    'RUN_ERROR',
  };

  group('event metadata and attribution', () {
    const decoder = EventDecoder();
    final encoder = EventEncoder();

    test('every event type preserves metadata through decode and SSE', () {
      for (final type in allTypes) {
        final event = decoder.decodeJson(
          _payload(type, attributed: attributedTypes.contains(type)),
        );
        expect(
          event.metadata,
          isA<Map<String, dynamic>>(),
          reason: type,
        );
        expect(
          event.toJson()['metadata'],
          isA<Map<String, dynamic>>(),
          reason: type,
        );

        final roundTrip = decoder.decodeSSE(encoder.encodeSSE(event));
        expect(roundTrip.metadata, event.metadata, reason: type);
      }
    });

    test('only the planned 24 event types expose typed attribution', () {
      for (final type in attributedTypes) {
        final event = decoder.decodeJson(_payload(type, attributed: true));
        expect(event.toJson()['subagentRunId'], 'child-$type', reason: type);
      }

      for (final type in const {
        'THINKING_START',
        'THINKING_CONTENT',
        'THINKING_END',
        'THINKING_TEXT_MESSAGE_START',
        'THINKING_TEXT_MESSAGE_CONTENT',
        'THINKING_TEXT_MESSAGE_END',
        'MESSAGES_SNAPSHOT',
        'RUN_STARTED',
        'RUN_FINISHED',
        'RUN_ERROR',
      }) {
        final event = decoder.decodeJson(_payload(type));
        expect(
          event.toJson().containsKey('subagentRunId'),
          isFalse,
          reason: type,
        );
      }
    });

    test('copyWith preserves, replaces, and clears added fields', () {
      const original = TextMessageStartEvent(
        messageId: 'message',
        metadata: {'one': 1},
        subagentRunId: 'child',
      );
      expect(original.copyWith().metadata, {'one': 1});
      expect(original.copyWith().subagentRunId, 'child');
      expect(original.copyWith(metadata: {'two': 2}).metadata, {'two': 2});
      expect(original.copyWith(subagentRunId: 'other').subagentRunId, 'other');
      expect(original.copyWith(metadata: null).metadata, isNull);
      expect(original.copyWith(subagentRunId: null).subagentRunId, isNull);
    });

    test('malformed metadata on a cipher event is scrubbed', () {
      final json = _payload('REASONING_ENCRYPTED_VALUE')
        ..['metadata'] = ['wrong'];
      expect(
        () => decoder.decodeJson(json),
        throwsA(
          isA<DecodingError>()
              .having((e) => e.actualValue, 'actualValue', isNull)
              .having(
                (e) => e.cause.toString(),
                'cause',
                isNot(contains('cipher')),
              ),
        ),
      );
    });
  });
}
