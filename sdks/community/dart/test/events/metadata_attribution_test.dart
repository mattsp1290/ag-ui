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

typedef _EventBuilder = BaseEvent Function(
  Metadata? metadata,
  String? subagentRunId,
);

final _attributedEventBuilders = <String, _EventBuilder>{
  'TEXT_MESSAGE_START': (metadata, subagentRunId) => TextMessageStartEvent(
        messageId: 'message',
        metadata: metadata,
        subagentRunId: subagentRunId,
      ),
  'TEXT_MESSAGE_CONTENT': (metadata, subagentRunId) => TextMessageContentEvent(
        messageId: 'message',
        delta: 'text',
        metadata: metadata,
        subagentRunId: subagentRunId,
      ),
  'TEXT_MESSAGE_END': (metadata, subagentRunId) => TextMessageEndEvent(
        messageId: 'message',
        metadata: metadata,
        subagentRunId: subagentRunId,
      ),
  'TEXT_MESSAGE_CHUNK': (metadata, subagentRunId) => TextMessageChunkEvent(
        messageId: 'message',
        delta: 'text',
        metadata: metadata,
        subagentRunId: subagentRunId,
      ),
  'TOOL_CALL_START': (metadata, subagentRunId) => ToolCallStartEvent(
        toolCallId: 'call',
        toolCallName: 'lookup',
        metadata: metadata,
        subagentRunId: subagentRunId,
      ),
  'TOOL_CALL_ARGS': (metadata, subagentRunId) => ToolCallArgsEvent(
        toolCallId: 'call',
        delta: '{}',
        metadata: metadata,
        subagentRunId: subagentRunId,
      ),
  'TOOL_CALL_END': (metadata, subagentRunId) => ToolCallEndEvent(
        toolCallId: 'call',
        metadata: metadata,
        subagentRunId: subagentRunId,
      ),
  'TOOL_CALL_CHUNK': (metadata, subagentRunId) => ToolCallChunkEvent(
        toolCallId: 'call',
        toolCallName: 'lookup',
        delta: '{}',
        metadata: metadata,
        subagentRunId: subagentRunId,
      ),
  'TOOL_CALL_RESULT': (metadata, subagentRunId) => ToolCallResultEvent(
        messageId: 'message',
        toolCallId: 'call',
        content: 'result',
        metadata: metadata,
        subagentRunId: subagentRunId,
      ),
  'STATE_SNAPSHOT': (metadata, subagentRunId) => StateSnapshotEvent(
        snapshot: {'state': true},
        metadata: metadata,
        subagentRunId: subagentRunId,
      ),
  'STATE_DELTA': (metadata, subagentRunId) => StateDeltaEvent(
        delta: const [],
        metadata: metadata,
        subagentRunId: subagentRunId,
      ),
  'ACTIVITY_SNAPSHOT': (metadata, subagentRunId) => ActivitySnapshotEvent(
        messageId: 'activity',
        activityType: 'PLAN',
        content: const <String, dynamic>{},
        metadata: metadata,
        subagentRunId: subagentRunId,
      ),
  'ACTIVITY_DELTA': (metadata, subagentRunId) => ActivityDeltaEvent(
        messageId: 'activity',
        activityType: 'PLAN',
        patch: const [],
        metadata: metadata,
        subagentRunId: subagentRunId,
      ),
  'RAW': (metadata, subagentRunId) => RawEvent(
        event: const <String, dynamic>{'provider': 'test'},
        metadata: metadata,
        subagentRunId: subagentRunId,
      ),
  'CUSTOM': (metadata, subagentRunId) => CustomEvent(
        name: 'custom',
        value: null,
        metadata: metadata,
        subagentRunId: subagentRunId,
      ),
  'STEP_STARTED': (metadata, subagentRunId) => StepStartedEvent(
        stepName: 'step',
        metadata: metadata,
        subagentRunId: subagentRunId,
      ),
  'STEP_FINISHED': (metadata, subagentRunId) => StepFinishedEvent(
        stepName: 'step',
        metadata: metadata,
        subagentRunId: subagentRunId,
      ),
  'REASONING_START': (metadata, subagentRunId) => ReasoningStartEvent(
        messageId: 'reasoning',
        metadata: metadata,
        subagentRunId: subagentRunId,
      ),
  'REASONING_MESSAGE_START': (metadata, subagentRunId) =>
      ReasoningMessageStartEvent(
        messageId: 'reasoning',
        metadata: metadata,
        subagentRunId: subagentRunId,
      ),
  'REASONING_MESSAGE_CONTENT': (metadata, subagentRunId) =>
      ReasoningMessageContentEvent(
        messageId: 'reasoning',
        delta: 'thinking',
        metadata: metadata,
        subagentRunId: subagentRunId,
      ),
  'REASONING_MESSAGE_END': (metadata, subagentRunId) =>
      ReasoningMessageEndEvent(
        messageId: 'reasoning',
        metadata: metadata,
        subagentRunId: subagentRunId,
      ),
  'REASONING_MESSAGE_CHUNK': (metadata, subagentRunId) =>
      ReasoningMessageChunkEvent(
        messageId: 'reasoning',
        delta: 'thinking',
        metadata: metadata,
        subagentRunId: subagentRunId,
      ),
  'REASONING_END': (metadata, subagentRunId) => ReasoningEndEvent(
        messageId: 'reasoning',
        metadata: metadata,
        subagentRunId: subagentRunId,
      ),
  'REASONING_ENCRYPTED_VALUE': (metadata, subagentRunId) =>
      ReasoningEncryptedValueEvent(
        subtype: ReasoningEncryptedValueSubtype.message,
        entityId: 'reasoning',
        encryptedValue: 'cipher',
        metadata: metadata,
        subagentRunId: subagentRunId,
      ),
};

final _metadataOnlyEventBuilders = <String, BaseEvent Function(Metadata?)>{
  'THINKING_START': (metadata) => ThinkingStartEvent(metadata: metadata),
  'THINKING_CONTENT': (metadata) =>
      ThinkingContentEvent(delta: 'thinking', metadata: metadata),
  'THINKING_END': (metadata) => ThinkingEndEvent(metadata: metadata),
  'THINKING_TEXT_MESSAGE_START': (metadata) =>
      ThinkingTextMessageStartEvent(metadata: metadata),
  'THINKING_TEXT_MESSAGE_CONTENT': (metadata) =>
      ThinkingTextMessageContentEvent(delta: 'thinking', metadata: metadata),
  'THINKING_TEXT_MESSAGE_END': (metadata) =>
      ThinkingTextMessageEndEvent(metadata: metadata),
  'MESSAGES_SNAPSHOT': (metadata) =>
      MessagesSnapshotEvent(messages: const [], metadata: metadata),
  'RUN_STARTED': (metadata) =>
      RunStartedEvent(threadId: 'thread', runId: 'run', metadata: metadata),
  'RUN_FINISHED': (metadata) =>
      RunFinishedEvent(threadId: 'thread', runId: 'run', metadata: metadata),
  'RUN_ERROR': (metadata) =>
      RunErrorEvent(message: 'failed', metadata: metadata),
};

AGUIValidationError _captureValidationError(void Function() action) {
  try {
    action();
  } on AGUIValidationError catch (error) {
    return error;
  }
  fail('Expected AGUIValidationError');
}

DecodingError _captureDecodingError(void Function() action) {
  try {
    action();
  } on DecodingError catch (error) {
    return error;
  }
  fail('Expected DecodingError');
}

void _expectNoSecrets(Object? value, Iterable<String> secrets) {
  final rendered = value.toString();
  for (final secret in secrets) {
    expect(rendered, isNot(contains(secret)));
  }
}

void _expectSanitizedValidationTree(
  AGUIValidationError error,
  Iterable<String> secrets,
) {
  expect(error.json, isNull);
  _expectNoSecrets(error.message, secrets);
  _expectNoSecrets(error.value, secrets);
  _expectNoSecrets(error, secrets);

  final cause = error.cause;
  if (cause is AGUIValidationError) {
    _expectSanitizedValidationTree(cause, secrets);
  } else {
    _expectNoSecrets(cause, secrets);
  }
}

void _expectScrubbedValidationError(
  AGUIValidationError error, {
  required String field,
  required Iterable<String> secrets,
  bool expectSafeCause = false,
}) {
  expect(error.field, field);
  expect(error.json, isNull);
  expect(error.value, 'List<String>');
  expect(error.message, contains('Expected String, got List<String>'));
  _expectSanitizedValidationTree(error, secrets);

  if (expectSafeCause) {
    final cause = error.cause;
    expect(cause, isA<AGUIValidationError>());
    _expectNoSecrets(cause, secrets);
  } else {
    expect(error.cause, isNull);
  }
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

    test(
      'all 24 attribution carriers cover construction, factories, copies, and SSE',
      () {
        expect(_attributedEventBuilders.keys.toSet(), attributedTypes);
        final metadata = <String, dynamic>{
          'carrier': 'event',
          'nullValue': null,
          'zero': 0,
          'falseValue': false,
        };

        for (final type in attributedTypes) {
          final builder = _attributedEventBuilders[type]!;
          final constructed = builder(metadata, 'constructed-$type');
          expect(constructed.metadata, same(metadata), reason: type);
          expect(
            (constructed as dynamic).subagentRunId,
            'constructed-$type',
            reason: type,
          );
          expect(constructed.toJson()['metadata'], metadata, reason: type);
          expect(
            constructed.toJson()['subagentRunId'],
            'constructed-$type',
            reason: type,
          );

          final direct = BaseEvent.fromJson(
            _payload(type, attributed: true),
          );
          final decoded = decoder.decodeJson(_payload(type, attributed: true));
          for (final event in [direct, decoded]) {
            expect(event.metadata, _payload(type)['metadata'], reason: type);
            expect(
              (event as dynamic).subagentRunId,
              'child-$type',
              reason: type,
            );
          }

          final dynamic dynamicEvent = constructed;
          final noOp = dynamicEvent.copyWith();
          expect(noOp.metadata, metadata, reason: '$type no-op metadata');
          expect(
            noOp.subagentRunId,
            'constructed-$type',
            reason: '$type no-op attribution',
          );
          expect(
            dynamicEvent.copyWith(metadata: {'replacement': type}).metadata,
            {'replacement': type},
            reason: '$type metadata replacement',
          );
          expect(
            dynamicEvent.copyWith(metadata: null).metadata,
            isNull,
            reason: '$type metadata clear',
          );
          expect(
            dynamicEvent
                .copyWith(subagentRunId: 'replacement-$type')
                .subagentRunId,
            'replacement-$type',
            reason: '$type attribution replacement',
          );
          expect(
            dynamicEvent.copyWith(subagentRunId: null).subagentRunId,
            isNull,
            reason: '$type attribution clear',
          );

          final roundTrip = decoder.decodeSSE(encoder.encodeSSE(constructed));
          expect(roundTrip.metadata, metadata, reason: '$type SSE metadata');
          expect(
            (roundTrip as dynamic).subagentRunId,
            'constructed-$type',
            reason: '$type SSE attribution',
          );
        }
      },
    );

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
        final event = decoder.decodeJson(_payload(type, attributed: true));
        expect(
          event.toJson().containsKey('subagentRunId'),
          isFalse,
          reason: type,
        );
      }
    });

    test('metadata-only events cover construction and copy lifecycle', () {
      for (final entry in _metadataOnlyEventBuilders.entries) {
        final metadata = <String, dynamic>{'event': entry.key};
        final constructed = entry.value(metadata);
        expect(constructed.metadata, same(metadata), reason: entry.key);
        expect(constructed.toJson()['metadata'], metadata, reason: entry.key);

        final dynamic dynamicEvent = constructed;
        expect(dynamicEvent.copyWith().metadata, metadata, reason: entry.key);
        expect(
          dynamicEvent.copyWith(metadata: {'replacement': entry.key}).metadata,
          {'replacement': entry.key},
          reason: entry.key,
        );
        expect(
          dynamicEvent.copyWith(metadata: null).metadata,
          isNull,
          reason: entry.key,
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

    test('direct cipher-bearing factories scrub malformed attribution', () {
      const cipher = 'direct-cipher-secret';
      const attributionSecret = 'direct-attribution-secret';
      final malformedAttribution = <String>[attributionSecret];

      final messageError = _captureValidationError(
        () => ReasoningMessage.fromJson({
          'id': 'reasoning',
          'role': 'reasoning',
          'content': 'thinking',
          'encryptedValue': cipher,
          'subagentRunId': malformedAttribution,
        }),
      );
      _expectScrubbedValidationError(
        messageError,
        field: 'subagentRunId',
        secrets: const [cipher, attributionSecret],
      );

      final eventError = _captureValidationError(
        () => ReasoningEncryptedValueEvent.fromJson({
          'type': 'REASONING_ENCRYPTED_VALUE',
          'subtype': 'message',
          'entityId': 'reasoning',
          'encryptedValue': cipher,
          'subagent_run_id': malformedAttribution,
        }),
      );
      _expectScrubbedValidationError(
        eventError,
        field: 'subagent_run_id',
        secrets: const [cipher, attributionSecret],
      );
    });

    test('public decoder does not retain malformed cipher attribution', () {
      const cipher = 'public-cipher-secret';
      const attributionSecret = 'public-attribution-secret';
      final json = _payload('REASONING_ENCRYPTED_VALUE')
        ..['encryptedValue'] = cipher
        ..['subagent_run_id'] = <String>[attributionSecret];

      final error = _captureDecodingError(() => decoder.decodeJson(json));
      expect(error.field, 'subagent_run_id');
      expect(error.actualValue, isNull);
      expect(error.cause, isA<AGUIValidationError>());
      _expectNoSecrets(error.message, const [cipher, attributionSecret]);
      _expectNoSecrets(error.actualValue, const [cipher, attributionSecret]);
      _expectSanitizedValidationTree(
        error.cause! as AGUIValidationError,
        const [cipher, attributionSecret],
      );
      _expectNoSecrets(error, const [cipher, attributionSecret]);
    });

    test('snapshot nesting keeps malformed cipher attribution scrubbed', () {
      const cipher = 'snapshot-cipher-secret';
      const attributionSecret = 'snapshot-attribution-secret';
      final error = _captureValidationError(
        () => MessagesSnapshotEvent.fromJson({
          'type': 'MESSAGES_SNAPSHOT',
          'messages': <Map<String, dynamic>>[
            {
              'id': 'reasoning',
              'role': 'reasoning',
              'encryptedValue': cipher,
              'subagentRunId': <String>[attributionSecret],
            },
          ],
        }),
      );

      _expectScrubbedValidationError(
        error,
        field: 'messages[0].subagentRunId',
        secrets: const [cipher, attributionSecret],
        expectSafeCause: true,
      );
    });

    test('run input nesting remains scrubbed at the public boundary', () {
      const cipher = 'run-input-cipher-secret';
      const attributionSecret = 'run-input-attribution-secret';
      final error = _captureDecodingError(
        () => decoder.decodeJson({
          'type': 'RUN_STARTED',
          'threadId': 'thread',
          'runId': 'run',
          'input': <String, dynamic>{
            'threadId': 'thread',
            'runId': 'run',
            'state': <String, dynamic>{},
            'messages': <Map<String, dynamic>>[
              {
                'id': 'reasoning',
                'role': 'reasoning',
                'encryptedValue': cipher,
                'subagentRunId': <String>[attributionSecret],
              },
            ],
            'tools': <Map<String, dynamic>>[],
            'context': <Map<String, dynamic>>[],
            'forwardedProps': <String, dynamic>{},
          },
        }),
      );

      expect(error.field, 'input.messages[0].subagentRunId');
      expect(error.actualValue, isNull);
      expect(error.cause, isA<AGUIValidationError>());
      _expectNoSecrets(error.message, const [cipher, attributionSecret]);
      _expectNoSecrets(error.actualValue, const [cipher, attributionSecret]);
      _expectSanitizedValidationTree(
        error.cause! as AGUIValidationError,
        const [cipher, attributionSecret],
      );
      _expectNoSecrets(error, const [cipher, attributionSecret]);
    });

    test('nested tool-call metadata errors do not expose ciphertext', () {
      const cipher = 'tool-call-cipher-secret';
      const metadataSecret = 'tool-call-metadata-secret';
      final error = _captureDecodingError(
        () => decoder.decodeJson({
          'type': 'MESSAGES_SNAPSHOT',
          'messages': <Map<String, dynamic>>[
            {
              'id': 'assistant',
              'role': 'assistant',
              'toolCalls': <Map<String, dynamic>>[
                {
                  'id': 'call',
                  'type': 'function',
                  'function': {'name': 'lookup', 'arguments': '{}'},
                  'encryptedValue': cipher,
                  'metadata': <String>[metadataSecret],
                },
              ],
            },
          ],
        }),
      );

      expect(error.field, 'messages[0].toolCalls[0].metadata');
      expect(error.actualValue, isNull);
      expect(error.cause, isA<AGUIValidationError>());
      _expectNoSecrets(error.message, const [cipher, metadataSecret]);
      _expectNoSecrets(error.actualValue, const [cipher, metadataSecret]);
      _expectSanitizedValidationTree(
        error.cause! as AGUIValidationError,
        const [cipher, metadataSecret],
      );
      _expectNoSecrets(error, const [cipher, metadataSecret]);
    });
  });
}
