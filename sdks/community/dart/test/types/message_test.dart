import 'package:test/test.dart';
import 'package:ag_ui/ag_ui.dart';

void main() {
  group('Message Types', () {
    group('DeveloperMessage', () {
      test('should serialize and deserialize correctly', () {
        final message = DeveloperMessage(
          id: 'msg_001',
          content: 'This is a developer message',
          name: 'dev_system',
        );

        final json = message.toJson();
        expect(json['id'], 'msg_001');
        expect(json['role'], 'developer');
        expect(json['content'], 'This is a developer message');
        expect(json['name'], 'dev_system');

        final decoded = DeveloperMessage.fromJson(json);
        expect(decoded.id, message.id);
        expect(decoded.content, message.content);
        expect(decoded.name, message.name);
        expect(decoded.role, MessageRole.developer);
      });

      test('should handle missing optional fields', () {
        final json = {
          'id': 'msg_002',
          'role': 'developer',
          'content': 'Minimal developer message',
        };

        final message = DeveloperMessage.fromJson(json);
        expect(message.id, 'msg_002');
        expect(message.content, 'Minimal developer message');
        expect(message.name, isNull);
      });

      test('should throw on missing required fields', () {
        final json = {
          'id': 'msg_003',
          'role': 'developer',
        };

        expect(
          () => DeveloperMessage.fromJson(json),
          throwsA(isA<AGUIValidationError>()),
        );
      });
    });

    group('AssistantMessage', () {
      test('should handle tool calls', () {
        final message = AssistantMessage(
          id: 'asst_001',
          content: 'I will help you with that',
          toolCalls: [
            ToolCall(
              id: 'call_001',
              function: FunctionCall(
                name: 'get_weather',
                arguments: '{"location": "New York"}',
              ),
            ),
          ],
        );

        final json = message.toJson();
        expect(json['id'], 'asst_001');
        expect(json['role'], 'assistant');
        expect(json['content'], 'I will help you with that');
        expect(json['toolCalls'], isA<List>());
        expect(json['toolCalls']!.length, 1);

        final decoded = AssistantMessage.fromJson(json);
        expect(decoded.id, message.id);
        expect(decoded.content, message.content);
        expect(decoded.toolCalls?.length, 1);
        expect(decoded.toolCalls![0].id, 'call_001');
        expect(decoded.toolCalls![0].function.name, 'get_weather');
      });

      test('should handle both camelCase and snake_case tool calls', () {
        final snakeCaseJson = {
          'id': 'asst_002',
          'role': 'assistant',
          'tool_calls': [
            {
              'id': 'call_002',
              'type': 'function',
              'function': {
                'name': 'search',
                'arguments': '{"query": "AG-UI"}',
              },
            },
          ],
        };

        final message = AssistantMessage.fromJson(snakeCaseJson);
        expect(message.toolCalls?.length, 1);
        expect(message.toolCalls![0].id, 'call_002');
      });
    });

    group('ToolMessage', () {
      test('should handle error field', () {
        final message = ToolMessage(
          id: 'tool_001',
          content: 'Tool execution failed',
          toolCallId: 'call_001',
          error: 'Connection timeout',
        );

        final json = message.toJson();
        expect(json['error'], 'Connection timeout');

        final decoded = ToolMessage.fromJson(json);
        expect(decoded.error, 'Connection timeout');
      });

      test('should handle both camelCase and snake_case tool_call_id', () {
        final snakeCaseJson = {
          'id': 'tool_002',
          'role': 'tool',
          'content': 'Result',
          'tool_call_id': 'call_002',
        };

        final message = ToolMessage.fromJson(snakeCaseJson);
        expect(message.toolCallId, 'call_002');
      });
    });

    group('Message Factory', () {
      test('should create correct message type based on role', () {
        final messages = [
          {'id': '1', 'role': 'developer', 'content': 'Dev msg'},
          {'id': '2', 'role': 'system', 'content': 'System msg'},
          {'id': '3', 'role': 'user', 'content': 'User msg'},
          {'id': '4', 'role': 'assistant', 'content': 'Assistant msg'},
          {
            'id': '5',
            'role': 'tool',
            'content': 'Tool result',
            'toolCallId': 'call_001'
          },
        ];

        final decoded = messages.map((json) => Message.fromJson(json)).toList();

        expect(decoded[0], isA<DeveloperMessage>());
        expect(decoded[1], isA<SystemMessage>());
        expect(decoded[2], isA<UserMessage>());
        expect(decoded[3], isA<AssistantMessage>());
        expect(decoded[4], isA<ToolMessage>());
      });

      test('should throw on invalid role', () {
        final json = {
          'id': 'invalid_001',
          'role': 'invalid_role',
          'content': 'Some content',
        };

        expect(
          () => Message.fromJson(json),
          throwsA(isA<AGUIValidationError>()),
        );
      });
    });

    group('ActivityMessage', () {
      test('MessageRole.fromString("activity") returns activity role', () {
        expect(MessageRole.fromString('activity'), MessageRole.activity);
      });

      test('round-trips activityType and content', () {
        final message = ActivityMessage(
          id: 'act_001',
          activityType: 'file_upload',
          activityContent: {'progress': 0.5, 'filename': 'data.csv'},
        );

        final json = message.toJson();
        expect(json['id'], 'act_001');
        expect(json['role'], 'activity');
        expect(json['activityType'], 'file_upload');
        expect(json['content'], {'progress': 0.5, 'filename': 'data.csv'});

        final decoded = ActivityMessage.fromJson(json);
        expect(decoded.id, 'act_001');
        expect(decoded.activityType, 'file_upload');
        expect(decoded.activityContent,
            {'progress': 0.5, 'filename': 'data.csv'});
      });

      test('Message.fromJson routes role=activity to ActivityMessage', () {
        final decoded = Message.fromJson({
          'id': 'act_002',
          'role': 'activity',
          'activityType': 'thinking',
          'content': <String, dynamic>{'note': 'x'},
        });
        expect(decoded, isA<ActivityMessage>());
      });

      test('copyWith overrides fields', () {
        final original = ActivityMessage(
          id: 'act_1',
          activityType: 'upload',
          activityContent: const {'progress': 0.1},
        );
        final copy = original.copyWith(
          id: 'act_2',
          activityType: 'download',
          activityContent: const {'progress': 0.9},
        );
        expect(copy.id, 'act_2');
        expect(copy.activityType, 'download');
        expect(copy.activityContent, {'progress': 0.9});
      });

      test('copyWith preserves fields when no overrides given', () {
        final original = ActivityMessage(
          id: 'act_1',
          activityType: 'upload',
          activityContent: const {'progress': 0.1},
        );
        final copy = original.copyWith();
        expect(copy.id, 'act_1');
        expect(copy.activityType, 'upload');
        expect(copy.activityContent, {'progress': 0.1});
      });
    });

    group('Unknown field tolerance', () {
      test('should ignore unknown fields in JSON', () {
        final json = {
          'id': 'msg_unknown',
          'role': 'user',
          'content': 'User message',
          'unknown_field': 'should be ignored',
          'another_unknown': {'nested': 'data'},
        };

        final message = UserMessage.fromJson(json);
        expect(message.id, 'msg_unknown');
        expect(message.content, 'User message');

        final serialized = message.toJson();
        expect(serialized.containsKey('unknown_field'), false);
        expect(serialized.containsKey('another_unknown'), false);
      });
    });

    group('ReasoningMessage', () {
      test('Message.fromJson routes role=reasoning to ReasoningMessage', () {
        final msg = Message.fromJson({
          'id': 'r1',
          'role': 'reasoning',
          'content': 'I reasoned about X',
          'thinking': 'step-by-step...',
        });
        expect(msg, isA<ReasoningMessage>());
        expect(msg.role, MessageRole.reasoning);
      });

      test('round-trips all fields', () {
        final msg = ReasoningMessage(
          id: 'r1',
          content: 'conclusion',
          thinking: 'step-by-step',
          encryptedValue: 'ENC==',
        );
        final json = msg.toJson();
        expect(json['role'], 'reasoning');
        expect(json['content'], 'conclusion');
        expect(json['thinking'], 'step-by-step');
        expect(json['encryptedValue'], 'ENC==');

        final decoded = ReasoningMessage.fromJson(json);
        expect(decoded.id, 'r1');
        expect(decoded.content, 'conclusion');
        expect(decoded.thinking, 'step-by-step');
        expect(decoded.encryptedValue, 'ENC==');
      });

      test('accepts absent optional fields', () {
        final msg = ReasoningMessage.fromJson({'role': 'reasoning'});
        expect(msg.id, isNull);
        expect(msg.content, isNull);
        expect(msg.thinking, isNull);
        expect(msg.encryptedValue, isNull);
        expect(msg.toJson().containsKey('thinking'), false);
        expect(msg.toJson().containsKey('encryptedValue'), false);
      });

      test('reads snake_case encrypted_value', () {
        final msg = ReasoningMessage.fromJson({
          'role': 'reasoning',
          'encrypted_value': 'SNAKE_ENC',
        });
        expect(msg.encryptedValue, 'SNAKE_ENC');
      });

      test('copyWith overrides fields', () {
        final original = ReasoningMessage(
          id: 'r1',
          content: 'old',
          thinking: 'think',
          encryptedValue: 'ENC',
        );
        final copy = original.copyWith(content: 'new', encryptedValue: 'ENC2');
        expect(copy.id, 'r1');
        expect(copy.content, 'new');
        expect(copy.thinking, 'think');
        expect(copy.encryptedValue, 'ENC2');
      });

      test('MESSAGES_SNAPSHOT list containing reasoning message decodes without throwing', () {
        final snapshot = [
          {'id': '1', 'role': 'user', 'content': 'hi'},
          {'id': '2', 'role': 'assistant', 'content': 'thinking...'},
          {'id': '3', 'role': 'reasoning', 'thinking': 'step 1', 'content': 'answer'},
          {'id': '4', 'role': 'assistant', 'content': 'done'},
        ];
        final messages = snapshot.map((j) => Message.fromJson(j)).toList();
        expect(messages[2], isA<ReasoningMessage>());
        expect((messages[2] as ReasoningMessage).thinking, 'step 1');
      });
    });

    group('encryptedValue on Message types', () {
      test('AssistantMessage round-trips encryptedValue', () {
        final msg = AssistantMessage(
          id: 'a1',
          content: 'hello',
          encryptedValue: 'ENC==',
        );
        final json = msg.toJson();
        expect(json['encryptedValue'], 'ENC==');

        final decoded = AssistantMessage.fromJson(json);
        expect(decoded.encryptedValue, 'ENC==');
      });

      test('UserMessage round-trips encryptedValue', () {
        final msg = UserMessage.fromJson({
          'id': 'u1',
          'role': 'user',
          'content': 'hi',
          'encryptedValue': 'EU==',
        });
        expect(msg.encryptedValue, 'EU==');
        expect(msg.toJson()['encryptedValue'], 'EU==');
      });

      test('ToolMessage round-trips encryptedValue', () {
        final msg = ToolMessage(
          id: 't1',
          content: 'result',
          toolCallId: 'call_1',
          encryptedValue: 'ET==',
        );
        final decoded = ToolMessage.fromJson(msg.toJson());
        expect(decoded.encryptedValue, 'ET==');
      });

      test('reads snake_case encrypted_value on AssistantMessage', () {
        final msg = AssistantMessage.fromJson({
          'id': 'a2',
          'role': 'assistant',
          'content': 'hi',
          'encrypted_value': 'SNAKE_ENC',
        });
        expect(msg.encryptedValue, 'SNAKE_ENC');
      });

      test('omits encryptedValue from toJson when null', () {
        final msg = AssistantMessage(id: 'a3', content: 'hi');
        expect(msg.toJson().containsKey('encryptedValue'), false);
      });

      test('copyWith threads encryptedValue through AssistantMessage', () {
        final original = AssistantMessage(
          id: 'a4',
          content: 'original',
          encryptedValue: 'ENC',
        );
        final copy = original.copyWith(content: 'updated');
        expect(copy.encryptedValue, 'ENC');

        final cleared = original.copyWith(encryptedValue: 'NEW');
        expect(cleared.encryptedValue, 'NEW');
      });
    });
  });
}