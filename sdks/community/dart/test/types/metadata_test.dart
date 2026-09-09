import 'package:ag_ui/ag_ui.dart';
import 'package:test/test.dart';

List<Message> _messagesForPropagation() {
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

  return [
    for (final payload in payloads)
      Message.fromJson(
        payload
          ..['metadata'] = <String, dynamic>{'role': payload['role']}
          ..['subagent_run_id'] = 'child-${payload['role']}',
      ),
  ];
}

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
      final emptyIncoming = <String, dynamic>{};
      expect(mergeMetadata(existing, emptyIncoming), existing);
      expect(emptyIncoming, isEmpty);
      expect(existing, {
        agUiMetadataKey: {'old': true},
        'keep': 1,
      });
    });

    test('retains ordinary __proto__ keys', () {
      final metadata = <String, dynamic>{
        '__proto__': {'safe': true},
      };
      expect(mergeMetadata(null, metadata)!['__proto__'], {'safe': true});
    });

    test('all message roles carry metadata and attribution', () {
      for (final message in _messagesForPropagation()) {
        expect(message.metadata, {'role': message.role.value});
        expect(message.subagentRunId, 'child-${message.role.value}');
        expect(
          message.toJson()['subagentRunId'],
          'child-${message.role.value}',
        );
        expect(message.toJson().containsKey('subagent_run_id'), isFalse);
        final dynamic dynamicMessage = message;
        final copied = dynamicMessage.copyWith() as Message;
        expect(copied.metadata, message.metadata);
        expect(copied.subagentRunId, message.subagentRunId);
        expect(
          dynamicMessage.copyWith(metadata: {'replacement': true}).metadata,
          {'replacement': true},
        );
        expect(dynamicMessage.copyWith(metadata: null).metadata, isNull);
        expect(
          dynamicMessage.copyWith(subagentRunId: 'replacement').subagentRunId,
          'replacement',
        );
        expect(
          dynamicMessage.copyWith(subagentRunId: null).subagentRunId,
          isNull,
        );
      }
    });

    test('direct constructors cover every message role', () {
      final messages = <Message>[
        const DeveloperMessage(
          id: 'developer',
          content: 'content',
          metadata: {'role': 'developer'},
          subagentRunId: 'child-developer',
        ),
        const SystemMessage(
          id: 'system',
          content: 'content',
          metadata: {'role': 'system'},
          subagentRunId: 'child-system',
        ),
        const AssistantMessage(
          id: 'assistant',
          content: 'content',
          metadata: {'role': 'assistant'},
          subagentRunId: 'child-assistant',
        ),
        const UserMessage.fromContent(
          id: 'user',
          messageContent: TextContent('content'),
          metadata: {'role': 'user'},
          subagentRunId: 'child-user',
        ),
        const ToolMessage(
          id: 'tool',
          content: 'content',
          toolCallId: 'call',
          metadata: {'role': 'tool'},
          subagentRunId: 'child-tool',
        ),
        const ActivityMessage(
          id: 'activity',
          activityType: 'progress',
          activityContent: {'done': false},
          metadata: {'role': 'activity'},
          subagentRunId: 'child-activity',
        ),
        const ReasoningMessage(
          id: 'reasoning',
          content: 'thinking',
          metadata: {'role': 'reasoning'},
          subagentRunId: 'child-reasoning',
        ),
      ];
      for (final message in messages) {
        expect(message.toJson()['metadata'], {'role': message.role.value});
        expect(
          message.toJson()['subagentRunId'],
          'child-${message.role.value}',
        );
      }
    });

    test('all message roles survive snapshots and nested run input', () {
      final messages = _messagesForPropagation();
      const decoder = EventDecoder();
      final encoder = EventEncoder();
      final snapshot = MessagesSnapshotEvent(
        messages: messages,
        metadata: {'scope': 'snapshot'},
      );
      final snapshotDecoded =
          decoder.decodeJson(snapshot.toJson()) as MessagesSnapshotEvent;
      final snapshotSse = decoder.decodeSSE(encoder.encodeSSE(snapshot))
          as MessagesSnapshotEvent;
      for (var i = 0; i < messages.length; i++) {
        expect(snapshotDecoded.messages[i].metadata, messages[i].metadata);
        expect(
          snapshotDecoded.messages[i].subagentRunId,
          messages[i].subagentRunId,
        );
        expect(snapshotSse.messages[i].metadata, messages[i].metadata);
        expect(
          snapshotSse.messages[i].subagentRunId,
          messages[i].subagentRunId,
        );
      }

      final input = RunAgentInput(
        threadId: 'thread',
        runId: 'run',
        messages: messages,
        tools: const [],
        context: const [],
      );
      final started = RunStartedEvent(
        threadId: 'thread',
        runId: 'run',
        input: input,
        metadata: {'scope': 'run'},
      );
      final startedDecoded =
          decoder.decodeJson(started.toJson()) as RunStartedEvent;
      for (var i = 0; i < messages.length; i++) {
        expect(
          startedDecoded.input!.messages[i].metadata,
          messages[i].metadata,
        );
        expect(
          startedDecoded.input!.messages[i].subagentRunId,
          messages[i].subagentRunId,
        );
      }
    });

    test('metadata and copy sentinel work on ToolCall and Tool', () {
      const call = ToolCall(
        id: 'call',
        function: FunctionCall(name: 'lookup', arguments: '{}'),
        metadata: {'trace': 'one'},
      );
      const secondCall = ToolCall(
        id: 'second',
        function: FunctionCall(name: 'other', arguments: '{}'),
        metadata: {'trace': 'two'},
      );
      expect(ToolCall.fromJson(call.toJson()).metadata, {'trace': 'one'});
      expect(call.copyWith().metadata, {'trace': 'one'});
      expect(call.copyWith(metadata: {'trace': 'two'}).metadata, {
        'trace': 'two',
      });
      expect(call.copyWith(metadata: null).metadata, isNull);
      const assistant = AssistantMessage(
        id: 'assistant',
        content: 'content',
        toolCalls: [call, secondCall],
      );
      final decodedAssistant = AssistantMessage.fromJson(assistant.toJson());
      expect(decodedAssistant.toolCalls![0].metadata, {'trace': 'one'});
      expect(decodedAssistant.toolCalls![1].metadata, {'trace': 'two'});

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
