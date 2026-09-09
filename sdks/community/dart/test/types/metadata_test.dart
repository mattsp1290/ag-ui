import 'package:ag_ui/ag_ui.dart';
import 'package:test/test.dart';

void main() {
  group('Metadata', () {
    test('is publicly exported and merges shallowly without mutation', () {
      final existing = <String, dynamic>{
        agUiMetadataKey: {'old': true},
        'keep': 1,
      };
      final incoming = <String, dynamic>{
        agUiMetadataKey: {'new': true},
        'nullValue': null,
        'zero': 0,
        'falseValue': false,
        'list': [2],
      };

      final merged = mergeMetadata(existing, incoming);

      expect(merged, {
        agUiMetadataKey: {'new': true},
        'keep': 1,
        'nullValue': null,
        'zero': 0,
        'falseValue': false,
        'list': [2],
      });
      expect(existing[agUiMetadataKey], {'old': true});
      expect(incoming[agUiMetadataKey], {'new': true});
      expect(mergeMetadata(existing, null), same(existing));
      expect(mergeMetadata(null, incoming), isNot(same(incoming)));
      expect(mergeMetadata(null, incoming), incoming);
      expect(mergeMetadata(existing, <String, dynamic>{}), isNotNull);
    });

    test('retains ordinary __proto__ keys', () {
      final metadata = <String, dynamic>{
        '__proto__': {'safe': true},
      };
      expect(mergeMetadata(null, metadata)!['__proto__'], {'safe': true});
    });

    test('all message roles carry metadata and attribution', () {
      final payloads = <Map<String, dynamic>>[
        {'id': 'developer', 'role': 'developer', 'content': 'content'},
        {'id': 'system', 'role': 'system', 'content': 'content'},
        {'id': 'assistant', 'role': 'assistant', 'content': 'content'},
        {'id': 'user', 'role': 'user', 'content': 'content'},
        {
          'id': 'tool',
          'role': 'tool',
          'content': 'content',
          'toolCallId': 'call',
        },
        {
          'id': 'activity',
          'role': 'activity',
          'activityType': 'progress',
          'content': {'done': false},
        },
        {'id': 'reasoning', 'role': 'reasoning', 'content': 'thinking'},
      ];

      for (final payload in payloads) {
        payload['metadata'] = <String, dynamic>{'role': payload['role']};
        payload['subagent_run_id'] = 'child-${payload['role']}';
        final message = Message.fromJson(payload);
        expect(message.metadata, {'role': payload['role']});
        expect(message.subagentRunId, 'child-${payload['role']}');
        expect(message.toJson()['subagentRunId'], 'child-${payload['role']}');
        expect(message.toJson().containsKey('subagent_run_id'), isFalse);
        final copied = message.copyWith() as Message;
        expect(copied.metadata, message.metadata);
        expect(copied.subagentRunId, message.subagentRunId);
      }
    });

    test('metadata and copy sentinel work on ToolCall and Tool', () {
      const call = ToolCall(
        id: 'call',
        function: FunctionCall(name: 'lookup', arguments: '{}'),
        metadata: {'trace': 'one'},
      );
      expect(ToolCall.fromJson(call.toJson()).metadata, {'trace': 'one'});
      expect(call.copyWith().metadata, {'trace': 'one'});
      expect(call.copyWith(metadata: {'trace': 'two'}).metadata, {
        'trace': 'two',
      });
      expect(call.copyWith(metadata: null).metadata, isNull);

      const tool = Tool(
        name: 'lookup',
        description: 'lookup',
        metadata: {'provider': 'test'},
      );
      expect(Tool.fromJson(tool.toJson()).metadata, {'provider': 'test'});
      expect(tool.copyWith().metadata, {'provider': 'test'});
      expect(tool.copyWith(metadata: null).metadata, isNull);
    });

    test('SimpleRunAgentInput preserves top-level metadata', () {
      final input = SimpleRunAgentInput(
        messages: [
          UserMessage(
            id: 'user',
            content: 'hello',
            metadata: {'source': 'test'},
            subagentRunId: 'child',
          ),
        ],
        metadata: {'request': 'one'},
      );
      final json = input.toJson();
      expect(json['metadata'], {'request': 'one'});
      expect(json['messages'].single['metadata'], {'source': 'test'});
      expect(json['messages'].single['subagentRunId'], 'child');
    });

    test(
      'malformed cipher-bearing metadata errors do not retain ciphertext',
      () {
        final json = <String, dynamic>{
          'id': 'reasoning',
          'role': 'reasoning',
          'encryptedValue': 'secret-cipher',
          'metadata': ['wrong'],
        };

        expect(
          () => ReasoningMessage.fromJson(json),
          throwsA(
            isA<AGUIValidationError>()
                .having((e) => e.json, 'json', isNull)
                .having((e) => e.cause, 'cause', isNull)
                .having((e) => e.value, 'value', 'List<String>')
                .having((e) => e.message, 'message', isNot(contains('secret'))),
          ),
        );
      },
    );
  });
}
