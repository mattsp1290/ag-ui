import 'package:test/test.dart';
import 'package:ag_ui/ag_ui.dart';

void main() {
  group('Event Types', () {
    group('TextMessageEvents', () {
      test('TextMessageStartEvent serialization', () {
        final event = TextMessageStartEvent(
          messageId: 'msg_001',
          role: TextMessageRole.assistant,
          timestamp: 1234567890,
        );

        final json = event.toJson();
        expect(json['type'], 'TEXT_MESSAGE_START');
        expect(json['messageId'], 'msg_001');
        expect(json['role'], 'assistant');
        expect(json['timestamp'], 1234567890);

        final decoded = TextMessageStartEvent.fromJson(json);
        expect(decoded.messageId, event.messageId);
        expect(decoded.role, event.role);
        expect(decoded.timestamp, event.timestamp);
      });

      test('TextMessageContentEvent validation', () {
        // Valid event with non-empty delta
        final validEvent = TextMessageContentEvent(
          messageId: 'msg_001',
          delta: 'Hello world',
        );
        expect(validEvent.delta, 'Hello world');

        // Invalid event with empty delta should throw
        final invalidJson = {
          'type': 'TEXT_MESSAGE_CONTENT',
          'messageId': 'msg_001',
          'delta': '',
        };

        expect(
          () => TextMessageContentEvent.fromJson(invalidJson),
          throwsA(isA<AGUIValidationError>()),
        );
      });

      test('TextMessageChunkEvent optional fields', () {
        final event = TextMessageChunkEvent(
          messageId: 'msg_001',
          role: TextMessageRole.user,
          delta: 'chunk content',
        );

        final json = event.toJson();
        expect(json['messageId'], 'msg_001');
        expect(json['role'], 'user');
        expect(json['delta'], 'chunk content');

        // Test with all fields null
        final minimalEvent = TextMessageChunkEvent();
        final minimalJson = minimalEvent.toJson();
        expect(minimalJson.containsKey('messageId'), false);
        expect(minimalJson.containsKey('role'), false);
        expect(minimalJson.containsKey('delta'), false);
      });
    });

    group('ToolCallEvents', () {
      test('ToolCallStartEvent with parent message', () {
        final event = ToolCallStartEvent(
          toolCallId: 'call_001',
          toolCallName: 'get_weather',
          parentMessageId: 'msg_001',
        );

        final json = event.toJson();
        expect(json['type'], 'TOOL_CALL_START');
        expect(json['toolCallId'], 'call_001');
        expect(json['toolCallName'], 'get_weather');
        expect(json['parentMessageId'], 'msg_001');

        final decoded = ToolCallStartEvent.fromJson(json);
        expect(decoded.toolCallId, event.toolCallId);
        expect(decoded.toolCallName, event.toolCallName);
        expect(decoded.parentMessageId, event.parentMessageId);
      });

      test('ToolCallResultEvent role field', () {
        final event = ToolCallResultEvent(
          messageId: 'msg_001',
          toolCallId: 'call_001',
          content: 'Weather: Sunny, 72°F',
          role: 'tool',
        );

        final json = event.toJson();
        expect(json['role'], 'tool');

        final decoded = ToolCallResultEvent.fromJson(json);
        expect(decoded.role, 'tool');
      });
    });

    group('StateEvents', () {
      test('StateSnapshotEvent with complex state', () {
        final complexState = {
          'counter': 42,
          'messages': ['msg1', 'msg2'],
          'metadata': {
            'timestamp': 1234567890,
            'user': 'test_user',
          },
        };

        final event = StateSnapshotEvent(snapshot: complexState);

        final json = event.toJson();
        expect(json['type'], 'STATE_SNAPSHOT');
        expect(json['snapshot'], complexState);

        final decoded = StateSnapshotEvent.fromJson(json);
        expect(decoded.snapshot, complexState);
      });

      test('StateDeltaEvent with JSON Patch operations', () {
        final delta = [
          {'op': 'add', 'path': '/foo', 'value': 'bar'},
          {'op': 'remove', 'path': '/baz'},
          {'op': 'replace', 'path': '/qux', 'value': 42},
        ];

        final event = StateDeltaEvent(delta: delta);

        final json = event.toJson();
        expect(json['type'], 'STATE_DELTA');
        expect(json['delta'], delta);

        final decoded = StateDeltaEvent.fromJson(json);
        expect(decoded.delta, delta);
      });

      test('MessagesSnapshotEvent with mixed message types', () {
        final messages = [
          UserMessage(id: '1', content: 'Hello'),
          AssistantMessage(id: '2', content: 'Hi there'),
          ToolMessage(
            id: '3',
            content: 'Result',
            toolCallId: 'call_001',
          ),
        ];

        final event = MessagesSnapshotEvent(messages: messages);

        final json = event.toJson();
        expect(json['type'], 'MESSAGES_SNAPSHOT');
        expect(json['messages'].length, 3);

        final decoded = MessagesSnapshotEvent.fromJson(json);
        expect(decoded.messages.length, 3);
        expect(decoded.messages[0], isA<UserMessage>());
        expect(decoded.messages[1], isA<AssistantMessage>());
        expect(decoded.messages[2], isA<ToolMessage>());
      });
    });

    group('LifecycleEvents', () {
      test('RunStartedEvent handles both camelCase and snake_case', () {
        // Test camelCase
        final camelJson = {
          'type': 'RUN_STARTED',
          'threadId': 'thread_001',
          'runId': 'run_001',
        };

        final camelEvent = RunStartedEvent.fromJson(camelJson);
        expect(camelEvent.threadId, 'thread_001');
        expect(camelEvent.runId, 'run_001');

        // Test snake_case
        final snakeJson = {
          'type': 'RUN_STARTED',
          'thread_id': 'thread_002',
          'run_id': 'run_002',
        };

        final snakeEvent = RunStartedEvent.fromJson(snakeJson);
        expect(snakeEvent.threadId, 'thread_002');
        expect(snakeEvent.runId, 'run_002');
      });

      test('RunFinishedEvent with result', () {
        final result = {'status': 'success', 'data': [1, 2, 3]};
        final event = RunFinishedEvent(
          threadId: 'thread_001',
          runId: 'run_001',
          result: result,
        );

        final json = event.toJson();
        expect(json['result'], result);

        final decoded = RunFinishedEvent.fromJson(json);
        expect(decoded.result, result);
      });

      test('RunErrorEvent with error code', () {
        final event = RunErrorEvent(
          message: 'Something went wrong',
          code: 'ERR_TIMEOUT',
        );

        final json = event.toJson();
        expect(json['message'], 'Something went wrong');
        expect(json['code'], 'ERR_TIMEOUT');

        final decoded = RunErrorEvent.fromJson(json);
        expect(decoded.message, event.message);
        expect(decoded.code, event.code);
      });

      test('StepEvents handle both camelCase and snake_case', () {
        // StepStartedEvent
        final stepStartSnake = {
          'type': 'STEP_STARTED',
          'step_name': 'processing',
        };

        final stepStart = StepStartedEvent.fromJson(stepStartSnake);
        expect(stepStart.stepName, 'processing');

        // StepFinishedEvent
        final stepEndCamel = {
          'type': 'STEP_FINISHED',
          'stepName': 'processing',
        };

        final stepEnd = StepFinishedEvent.fromJson(stepEndCamel);
        expect(stepEnd.stepName, 'processing');
      });
    });

    group('Event Factory', () {
      test('should create correct event type based on type field', () {
        final eventJsons = [
          {'type': 'TEXT_MESSAGE_START', 'messageId': 'msg_001'},
          {'type': 'TOOL_CALL_START', 'toolCallId': 'call_001', 'toolCallName': 'test'},
          {'type': 'STATE_SNAPSHOT', 'snapshot': {}},
          {'type': 'RUN_STARTED', 'threadId': 'thread_001', 'runId': 'run_001'},
          {'type': 'THINKING_START'},
          {'type': 'CUSTOM', 'name': 'my_event', 'value': 'data'},
        ];

        final events = eventJsons.map((json) => BaseEvent.fromJson(json)).toList();

        expect(events[0], isA<TextMessageStartEvent>());
        expect(events[1], isA<ToolCallStartEvent>());
        expect(events[2], isA<StateSnapshotEvent>());
        expect(events[3], isA<RunStartedEvent>());
        expect(events[4], isA<ThinkingStartEvent>());
        expect(events[5], isA<CustomEvent>());
      });

      test('should throw on invalid event type', () {
        final json = {
          'type': 'INVALID_EVENT_TYPE',
          'data': 'some data',
        };

        expect(
          () => BaseEvent.fromJson(json),
          throwsArgumentError,
        );
      });
    });

    group('ThinkingEvents', () {
      test('ThinkingStartEvent with title', () {
        final event = ThinkingStartEvent(title: 'Processing request');

        final json = event.toJson();
        expect(json['type'], 'THINKING_START');
        expect(json['title'], 'Processing request');

        final decoded = ThinkingStartEvent.fromJson(json);
        expect(decoded.title, 'Processing request');
      });

      test('ThinkingTextMessageContentEvent delta validation', () {
        final invalidJson = {
          'type': 'THINKING_TEXT_MESSAGE_CONTENT',
          'delta': '',
        };

        expect(
          () => ThinkingTextMessageContentEvent.fromJson(invalidJson),
          throwsA(isA<AGUIValidationError>()),
        );
      });
    });

    group('Raw and Custom Events', () {
      test('RawEvent with source', () {
        final rawEventData = {
          'original': 'event',
          'data': [1, 2, 3],
        };

        final event = RawEvent(
          event: rawEventData,
          source: 'external_api',
        );

        final json = event.toJson();
        expect(json['event'], rawEventData);
        expect(json['source'], 'external_api');

        final decoded = RawEvent.fromJson(json);
        expect(decoded.event, rawEventData);
        expect(decoded.source, 'external_api');
      });

      test('CustomEvent with complex value', () {
        final customValue = {
          'action': 'update_ui',
          'parameters': {'theme': 'dark', 'language': 'en'},
        };

        final event = CustomEvent(
          name: 'ui_config_change',
          value: customValue,
        );

        final json = event.toJson();
        expect(json['name'], 'ui_config_change');
        expect(json['value'], customValue);

        final decoded = CustomEvent.fromJson(json);
        expect(decoded.name, 'ui_config_change');
        expect(decoded.value, customValue);
      });
    });

    group('ActivityEvents', () {
      test('ActivitySnapshotEvent serialization round-trip', () {
        final content = {
          'title': 'Processing',
          'progress': 0.5,
          'steps': ['fetch', 'parse'],
        };

        final event = ActivitySnapshotEvent(
          messageId: 'msg_001',
          activityType: 'task.run',
          content: content,
          replace: false,
        );

        final json = event.toJson();
        expect(json['type'], 'ACTIVITY_SNAPSHOT');
        expect(json['messageId'], 'msg_001');
        expect(json['activityType'], 'task.run');
        expect(json['content'], content);
        expect(json['replace'], false);

        final decoded = ActivitySnapshotEvent.fromJson(json);
        expect(decoded.messageId, 'msg_001');
        expect(decoded.activityType, 'task.run');
        expect(decoded.content, content);
        expect(decoded.replace, false);
      });

      test('ActivitySnapshotEvent defaults replace to true', () {
        final json = {
          'type': 'ACTIVITY_SNAPSHOT',
          'messageId': 'msg_001',
          'activityType': 'task.run',
          'content': {'foo': 'bar'},
        };

        final decoded = ActivitySnapshotEvent.fromJson(json);
        expect(decoded.replace, true);
      });

      test('ActivitySnapshotEvent accepts snake_case (Python server)', () {
        final pythonJson = {
          'type': 'ACTIVITY_SNAPSHOT',
          'message_id': 'msg_002',
          'activity_type': 'task.run',
          'content': 'hello',
          'replace': true,
        };

        final decoded = ActivitySnapshotEvent.fromJson(pythonJson);
        expect(decoded.messageId, 'msg_002');
        expect(decoded.activityType, 'task.run');
        expect(decoded.content, 'hello');
        expect(decoded.replace, true);
      });

      test('ActivityDeltaEvent serialization round-trip', () {
        final patch = [
          {'op': 'replace', 'path': '/progress', 'value': 0.75},
          {'op': 'add', 'path': '/steps/-', 'value': 'finalize'},
        ];

        final event = ActivityDeltaEvent(
          messageId: 'msg_001',
          activityType: 'task.run',
          patch: patch,
        );

        final json = event.toJson();
        expect(json['type'], 'ACTIVITY_DELTA');
        expect(json['messageId'], 'msg_001');
        expect(json['activityType'], 'task.run');
        expect(json['patch'], patch);

        final decoded = ActivityDeltaEvent.fromJson(json);
        expect(decoded.messageId, 'msg_001');
        expect(decoded.activityType, 'task.run');
        expect(decoded.patch, patch);
      });

      test('ActivityDeltaEvent accepts snake_case (Python server)', () {
        final pythonJson = {
          'type': 'ACTIVITY_DELTA',
          'message_id': 'msg_003',
          'activity_type': 'task.run',
          'patch': [
            {'op': 'replace', 'path': '/x', 'value': 1},
          ],
        };

        final decoded = ActivityDeltaEvent.fromJson(pythonJson);
        expect(decoded.messageId, 'msg_003');
        expect(decoded.activityType, 'task.run');
        expect(decoded.patch.length, 1);
      });

      test('Activity events dispatch via BaseEvent.fromJson', () {
        final snapshot = BaseEvent.fromJson({
          'type': 'ACTIVITY_SNAPSHOT',
          'messageId': 'm',
          'activityType': 't',
          'content': null,
        });
        expect(snapshot, isA<ActivitySnapshotEvent>());

        final delta = BaseEvent.fromJson({
          'type': 'ACTIVITY_DELTA',
          'messageId': 'm',
          'activityType': 't',
          'patch': <dynamic>[],
        });
        expect(delta, isA<ActivityDeltaEvent>());
      });

      test('ActivitySnapshotEvent copyWith preserves untouched fields', () {
        final original = ActivitySnapshotEvent(
          messageId: 'msg_001',
          activityType: 'task.run',
          content: 'original',
        );

        final updated = original.copyWith(content: 'new');
        expect(updated.messageId, original.messageId);
        expect(updated.activityType, original.activityType);
        expect(updated.content, 'new');
        expect(updated.replace, original.replace);
      });
    });

    group('ReasoningEvents', () {
      test('ReasoningStartEvent serialization round-trip', () {
        final event = ReasoningStartEvent(messageId: 'msg_r1');

        final json = event.toJson();
        expect(json['type'], 'REASONING_START');
        expect(json['messageId'], 'msg_r1');

        final decoded = ReasoningStartEvent.fromJson(json);
        expect(decoded.messageId, 'msg_r1');
      });

      test('ReasoningStartEvent accepts snake_case', () {
        final decoded = ReasoningStartEvent.fromJson({
          'type': 'REASONING_START',
          'message_id': 'msg_r1',
        });
        expect(decoded.messageId, 'msg_r1');
      });

      test('ReasoningMessageStartEvent default role is reasoning', () {
        final event = ReasoningMessageStartEvent(messageId: 'msg_r2');
        expect(event.role, ReasoningMessageRole.reasoning);

        final json = event.toJson();
        expect(json['type'], 'REASONING_MESSAGE_START');
        expect(json['role'], 'reasoning');

        final decoded = ReasoningMessageStartEvent.fromJson(json);
        expect(decoded.role, ReasoningMessageRole.reasoning);
        expect(decoded.messageId, 'msg_r2');
      });

      test('ReasoningMessageContentEvent serialization round-trip', () {
        final event = ReasoningMessageContentEvent(
          messageId: 'msg_r3',
          delta: 'thinking step',
        );

        final json = event.toJson();
        expect(json['type'], 'REASONING_MESSAGE_CONTENT');
        expect(json['delta'], 'thinking step');

        final decoded = ReasoningMessageContentEvent.fromJson(json);
        expect(decoded.messageId, 'msg_r3');
        expect(decoded.delta, 'thinking step');
      });

      test('ReasoningMessageEndEvent serialization round-trip', () {
        final event = ReasoningMessageEndEvent(messageId: 'msg_r4');

        final json = event.toJson();
        expect(json['type'], 'REASONING_MESSAGE_END');

        final decoded = ReasoningMessageEndEvent.fromJson(json);
        expect(decoded.messageId, 'msg_r4');
      });

      test('ReasoningMessageChunkEvent allows all-optional payload', () {
        final empty = ReasoningMessageChunkEvent();
        final emptyJson = empty.toJson();
        expect(emptyJson['type'], 'REASONING_MESSAGE_CHUNK');
        expect(emptyJson.containsKey('messageId'), false);
        expect(emptyJson.containsKey('delta'), false);

        final decoded = ReasoningMessageChunkEvent.fromJson(emptyJson);
        expect(decoded.messageId, isNull);
        expect(decoded.delta, isNull);

        final populated = ReasoningMessageChunkEvent(
          messageId: 'msg_r5',
          delta: 'partial',
        );
        final pjson = populated.toJson();
        expect(pjson['messageId'], 'msg_r5');
        expect(pjson['delta'], 'partial');
      });

      test('ReasoningEndEvent serialization round-trip', () {
        final event = ReasoningEndEvent(messageId: 'msg_r6');

        final json = event.toJson();
        expect(json['type'], 'REASONING_END');

        final decoded = ReasoningEndEvent.fromJson(json);
        expect(decoded.messageId, 'msg_r6');
      });

      test('ReasoningEncryptedValueEvent supports both subtypes', () {
        final tool = ReasoningEncryptedValueEvent(
          subtype: ReasoningEncryptedValueSubtype.toolCall,
          entityId: 'tc_1',
          encryptedValue: 'cipher-1',
        );
        final toolJson = tool.toJson();
        expect(toolJson['type'], 'REASONING_ENCRYPTED_VALUE');
        expect(toolJson['subtype'], 'tool-call');
        expect(toolJson['entityId'], 'tc_1');
        expect(toolJson['encryptedValue'], 'cipher-1');

        final decodedTool = ReasoningEncryptedValueEvent.fromJson(toolJson);
        expect(decodedTool.subtype, ReasoningEncryptedValueSubtype.toolCall);
        expect(decodedTool.entityId, 'tc_1');
        expect(decodedTool.encryptedValue, 'cipher-1');

        final msg = ReasoningEncryptedValueEvent(
          subtype: ReasoningEncryptedValueSubtype.message,
          entityId: 'm_1',
          encryptedValue: 'cipher-2',
        );
        expect(msg.toJson()['subtype'], 'message');
      });

      test('ReasoningEncryptedValueEvent accepts snake_case', () {
        final decoded = ReasoningEncryptedValueEvent.fromJson({
          'type': 'REASONING_ENCRYPTED_VALUE',
          'subtype': 'tool-call',
          'entity_id': 'tc_2',
          'encrypted_value': 'cipher-3',
        });
        expect(decoded.subtype, ReasoningEncryptedValueSubtype.toolCall);
        expect(decoded.entityId, 'tc_2');
        expect(decoded.encryptedValue, 'cipher-3');
      });

      test('ReasoningEncryptedValueSubtype.fromString rejects invalid input',
          () {
        expect(
          () => ReasoningEncryptedValueSubtype.fromString('bogus'),
          throwsA(isA<ArgumentError>()),
        );
      });

      test('Reasoning events dispatch via BaseEvent.fromJson', () {
        final cases = <Map<String, dynamic>, Type>{
          {'type': 'REASONING_START', 'messageId': 'm'}:
              ReasoningStartEvent,
          {'type': 'REASONING_MESSAGE_START', 'messageId': 'm'}:
              ReasoningMessageStartEvent,
          {'type': 'REASONING_MESSAGE_CONTENT', 'messageId': 'm', 'delta': 'd'}:
              ReasoningMessageContentEvent,
          {'type': 'REASONING_MESSAGE_END', 'messageId': 'm'}:
              ReasoningMessageEndEvent,
          {'type': 'REASONING_MESSAGE_CHUNK'}: ReasoningMessageChunkEvent,
          {'type': 'REASONING_END', 'messageId': 'm'}: ReasoningEndEvent,
          {
            'type': 'REASONING_ENCRYPTED_VALUE',
            'subtype': 'message',
            'entityId': 'e',
            'encryptedValue': 'v',
          }: ReasoningEncryptedValueEvent,
        };

        cases.forEach((json, type) {
          final event = BaseEvent.fromJson(json);
          expect(event.runtimeType, type, reason: 'for $json');
        });
      });
    });
  });
}