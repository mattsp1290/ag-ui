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

      test('TextMessage* events accept snake_case (Python server)', () {
        final start = TextMessageStartEvent.fromJson({
          'type': 'TEXT_MESSAGE_START',
          'message_id': 'msg_001',
          'role': 'assistant',
        });
        expect(start.messageId, 'msg_001');

        final content = TextMessageContentEvent.fromJson({
          'type': 'TEXT_MESSAGE_CONTENT',
          'message_id': 'msg_001',
          'delta': 'hello',
        });
        expect(content.messageId, 'msg_001');
        expect(content.delta, 'hello');

        final end = TextMessageEndEvent.fromJson({
          'type': 'TEXT_MESSAGE_END',
          'message_id': 'msg_001',
        });
        expect(end.messageId, 'msg_001');

        final chunk = TextMessageChunkEvent.fromJson({
          'type': 'TEXT_MESSAGE_CHUNK',
          'message_id': 'msg_001',
          'delta': 'partial',
        });
        expect(chunk.messageId, 'msg_001');
        expect(chunk.delta, 'partial');
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

      test('TextMessageRole.fromString throws on unknown values', () {
        // Aligned with `ReasoningMessageRole.fromString` — unknown wire
        // values throw at the enum so direct callers see a visible
        // failure mode. Wire decoding still succeeds via the factory's
        // absorb (see the `falls back to assistant` test below).
        expect(
          () => TextMessageRole.fromString('bogus'),
          throwsA(isA<ArgumentError>()),
        );
      });

      test(
          'TextMessageStartEvent falls back to assistant for an unknown '
          'role (forward-compat, no stream tear-down)', () {
        final decoded = TextMessageStartEvent.fromJson({
          'type': 'TEXT_MESSAGE_START',
          'messageId': 'msg_001',
          'role': 'bogus',
        });
        expect(decoded.role, TextMessageRole.assistant);
        expect(decoded.messageId, 'msg_001');
      });

      test(
          'TextMessageChunkEvent falls back to assistant for an unknown '
          'role (forward-compat parity with TextMessageStartEvent)', () {
        final decoded = TextMessageChunkEvent.fromJson({
          'type': 'TEXT_MESSAGE_CHUNK',
          'messageId': 'msg_001',
          'role': 'bogus',
          'delta': 'partial',
        });
        expect(decoded.role, TextMessageRole.assistant);
        expect(decoded.messageId, 'msg_001');
        expect(decoded.delta, 'partial');
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

      test('ToolCall* events accept snake_case (Python server)', () {
        final start = ToolCallStartEvent.fromJson({
          'type': 'TOOL_CALL_START',
          'tool_call_id': 'call_001',
          'tool_call_name': 'get_weather',
          'parent_message_id': 'msg_001',
        });
        expect(start.toolCallId, 'call_001');
        expect(start.toolCallName, 'get_weather');
        expect(start.parentMessageId, 'msg_001');

        final args = ToolCallArgsEvent.fromJson({
          'type': 'TOOL_CALL_ARGS',
          'tool_call_id': 'call_001',
          'delta': '{"q":"x"}',
        });
        expect(args.toolCallId, 'call_001');

        final end = ToolCallEndEvent.fromJson({
          'type': 'TOOL_CALL_END',
          'tool_call_id': 'call_001',
        });
        expect(end.toolCallId, 'call_001');

        final chunk = ToolCallChunkEvent.fromJson({
          'type': 'TOOL_CALL_CHUNK',
          'tool_call_id': 'call_001',
          'tool_call_name': 'get_weather',
          'parent_message_id': 'msg_001',
          'delta': '{',
        });
        expect(chunk.toolCallId, 'call_001');
        expect(chunk.toolCallName, 'get_weather');
        expect(chunk.parentMessageId, 'msg_001');

        final result = ToolCallResultEvent.fromJson({
          'type': 'TOOL_CALL_RESULT',
          'message_id': 'msg_001',
          'tool_call_id': 'call_001',
          'content': '72F sunny',
          'role': 'tool',
        });
        expect(result.messageId, 'msg_001');
        expect(result.toolCallId, 'call_001');
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

      test('should throw AGUIValidationError on invalid event type', () {
        // The factory wraps `EventType.fromString`'s raw `ArgumentError`
        // as `AGUIValidationError` so direct callers see the same error
        // surface as every other validation failure. Through the public
        // `EventDecoder` pipeline this surfaces as `DecodingError` —
        // see `event_decoding_integration_test.dart` ("validates
        // required fields strictly", invalid event type case).
        final json = {
          'type': 'INVALID_EVENT_TYPE',
          'data': 'some data',
        };

        expect(
          () => BaseEvent.fromJson(json),
          throwsA(isA<AGUIValidationError>()),
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

      test('deprecated ThinkingContentEvent still round-trips', () {
        // Locks in the backward-compat contract on the deprecation:
        // decoding/encoding must keep working until the planned removal.
        // ignore: deprecated_member_use_from_same_package
        final original = ThinkingContentEvent(delta: 'still works');
        final json = original.toJson();
        expect(json['type'], 'THINKING_CONTENT');
        expect(json['delta'], 'still works');

        // ignore: deprecated_member_use_from_same_package
        final decoded = ThinkingContentEvent.fromJson(json);
        expect(decoded.delta, 'still works');
      });

      test('EventDecoder still decodes deprecated THINKING_CONTENT', () {
        // Backs the CHANGELOG promise that the deprecated path remains
        // decodable end-to-end through the public decoder boundary.
        const decoder = EventDecoder();

        final event = decoder.decodeJson({
          'type': 'THINKING_CONTENT',
          'delta': 'legacy payload',
        });

        // ignore: deprecated_member_use_from_same_package
        expect(event, isA<ThinkingContentEvent>());
        // ignore: deprecated_member_use_from_same_package
        expect((event as ThinkingContentEvent).delta, 'legacy payload');
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
        expect((snapshot as ActivitySnapshotEvent).content, isNull);

        final delta = BaseEvent.fromJson({
          'type': 'ACTIVITY_DELTA',
          'messageId': 'm',
          'activityType': 't',
          'patch': <dynamic>[],
        });
        expect(delta, isA<ActivityDeltaEvent>());
      });

      test('ActivitySnapshotEvent rejects missing content key', () {
        // Mirrors the `StateSnapshotEvent` / `RawEvent` contract: the
        // payload field may be any JSON shape (including `null`) but the
        // KEY must be present. Distinguishing missing-key from
        // explicit-null is the whole point of this check.
        expect(
          () => ActivitySnapshotEvent.fromJson({
            'type': 'ACTIVITY_SNAPSHOT',
            'messageId': 'msg_001',
            'activityType': 'task.run',
          }),
          throwsA(isA<AGUIValidationError>()),
        );
      });

      test('ActivitySnapshotEvent accepts explicit-null content', () {
        // The companion to "rejects missing content key": an explicit
        // `null` is a valid wire payload (Python's `content: Any`
        // permits None) and must round-trip without error.
        final decoded = ActivitySnapshotEvent.fromJson({
          'type': 'ACTIVITY_SNAPSHOT',
          'messageId': 'msg_001',
          'activityType': 'task.run',
          'content': null,
        });
        expect(decoded.content, isNull);
      });

      test('ActivitySnapshotEvent.copyWith(content: null) clears content', () {
        // The factory contract permits explicit-null `content`, and so
        // must `copyWith` — distinguishing "argument omitted" from
        // "argument explicitly set to null" via the
        // `_unsetCopyWith` sentinel.
        final original = ActivitySnapshotEvent(
          messageId: 'msg_001',
          activityType: 'task.run',
          content: {'progress': 0.25},
        );
        // Omitted content keeps the existing value.
        final keep = original.copyWith();
        expect(keep.content, equals({'progress': 0.25}));

        // Explicit-null clears the content.
        final cleared = original.copyWith(content: null);
        expect(cleared.content, isNull);
      });

      test('ActivitySnapshotEvent rejects missing messageId', () {
        expect(
          () => ActivitySnapshotEvent.fromJson({
            'type': 'ACTIVITY_SNAPSHOT',
            'activityType': 'task.run',
            'content': null,
          }),
          throwsA(isA<AGUIValidationError>()),
        );
      });

      test('ActivityDeltaEvent rejects missing messageId', () {
        expect(
          () => ActivityDeltaEvent.fromJson({
            'type': 'ACTIVITY_DELTA',
            'activityType': 'task.run',
            'patch': <dynamic>[],
          }),
          throwsA(isA<AGUIValidationError>()),
        );
      });

      test('ActivityDeltaEvent rejects missing activityType', () {
        expect(
          () => ActivityDeltaEvent.fromJson({
            'type': 'ACTIVITY_DELTA',
            'messageId': 'msg_001',
            'patch': <dynamic>[],
          }),
          throwsA(isA<AGUIValidationError>()),
        );
      });

      test('ActivityDeltaEvent rejects missing patch', () {
        expect(
          () => ActivityDeltaEvent.fromJson({
            'type': 'ACTIVITY_DELTA',
            'messageId': 'msg_001',
            'activityType': 'task.run',
          }),
          throwsA(isA<AGUIValidationError>()),
        );
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

      test('ReasoningMessageStartEvent accepts snake_case', () {
        final decoded = ReasoningMessageStartEvent.fromJson({
          'type': 'REASONING_MESSAGE_START',
          'message_id': 'msg_r2',
          'role': 'reasoning',
        });
        expect(decoded.messageId, 'msg_r2');
        expect(decoded.role, ReasoningMessageRole.reasoning);
      });

      test('ReasoningMessageContentEvent accepts snake_case', () {
        final decoded = ReasoningMessageContentEvent.fromJson({
          'type': 'REASONING_MESSAGE_CONTENT',
          'message_id': 'msg_r3',
          'delta': 'thinking step',
        });
        expect(decoded.messageId, 'msg_r3');
        expect(decoded.delta, 'thinking step');
      });

      test('ReasoningMessageEndEvent accepts snake_case', () {
        final decoded = ReasoningMessageEndEvent.fromJson({
          'type': 'REASONING_MESSAGE_END',
          'message_id': 'msg_r4',
        });
        expect(decoded.messageId, 'msg_r4');
      });

      test('ReasoningEndEvent accepts snake_case', () {
        final decoded = ReasoningEndEvent.fromJson({
          'type': 'REASONING_END',
          'message_id': 'msg_r6',
        });
        expect(decoded.messageId, 'msg_r6');
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

      test('ReasoningMessageRole.fromString rejects invalid input', () {
        expect(
          () => ReasoningMessageRole.fromString('bogus'),
          throwsA(isA<ArgumentError>()),
        );
      });

      test(
          'ReasoningMessageStartEvent falls back to `reasoning` for an '
          'unknown role (forward-compat, no stream tear-down)', () {
        final decoded = ReasoningMessageStartEvent.fromJson({
          'type': 'REASONING_MESSAGE_START',
          'messageId': 'msg_r2',
          'role': 'bogus',
        });
        expect(decoded.role, ReasoningMessageRole.reasoning);
        expect(decoded.messageId, 'msg_r2');
      });

      test('ReasoningMessageStartEvent rejects missing role (parity with TS/Python)',
          () {
        // The canonical TypeScript and Python schemas both mark `role` as
        // required on REASONING_MESSAGE_START. A producer bug that drops
        // the field must surface as a protocol violation here, not be
        // silently coerced to `reasoning` (which would let malformed
        // payloads pass undetected and diverge from the reference SDKs).
        expect(
          () => ReasoningMessageStartEvent.fromJson({
            'type': 'REASONING_MESSAGE_START',
            'messageId': 'msg_r2',
          }),
          throwsA(isA<AGUIValidationError>()),
        );
      });

      test('ReasoningMessageChunkEvent accepts snake_case', () {
        final decoded = ReasoningMessageChunkEvent.fromJson({
          'type': 'REASONING_MESSAGE_CHUNK',
          'message_id': 'msg_r5',
          'delta': 'partial',
        });

        expect(decoded.messageId, 'msg_r5');
        expect(decoded.delta, 'partial');
      });

      test('ReasoningMessageContentEvent rejects missing delta', () {
        expect(
          () => ReasoningMessageContentEvent.fromJson({
            'type': 'REASONING_MESSAGE_CONTENT',
            'messageId': 'msg_r3',
          }),
          throwsA(isA<AGUIValidationError>()),
        );
      });

      test('ReasoningMessageContentEvent rejects empty delta', () {
        // Mirrors the TextMessageContentEvent / ThinkingContentEvent factory
        // contract — empty delta is rejected inside fromJson, not only later
        // by EventDecoder.validate.
        expect(
          () => ReasoningMessageContentEvent.fromJson({
            'type': 'REASONING_MESSAGE_CONTENT',
            'messageId': 'msg_r3',
            'delta': '',
          }),
          throwsA(isA<AGUIValidationError>()),
        );
      });

      test('ReasoningEncryptedValueEvent rejects missing subtype', () {
        expect(
          () => ReasoningEncryptedValueEvent.fromJson({
            'type': 'REASONING_ENCRYPTED_VALUE',
            'entityId': 'tc_1',
            'encryptedValue': 'cipher-1',
          }),
          throwsA(isA<AGUIValidationError>()),
        );
      });

      test('ReasoningEncryptedValueEvent rejects missing entityId', () {
        expect(
          () => ReasoningEncryptedValueEvent.fromJson({
            'type': 'REASONING_ENCRYPTED_VALUE',
            'subtype': 'message',
            'encryptedValue': 'cipher',
          }),
          throwsA(isA<AGUIValidationError>()),
        );
      });

      test('ReasoningEncryptedValueEvent rejects missing encryptedValue', () {
        expect(
          () => ReasoningEncryptedValueEvent.fromJson({
            'type': 'REASONING_ENCRYPTED_VALUE',
            'subtype': 'message',
            'entityId': 'msg_1',
          }),
          throwsA(isA<AGUIValidationError>()),
        );
      });

      test('ReasoningEncryptedValueEvent rejects unknown subtype', () {
        // Pins the dartdoc contract: an unknown `subtype` must surface
        // to direct factory callers as `AGUIValidationError` (not as
        // the raw `ArgumentError` that the enum itself throws). The
        // matching wire→DecodingError contract is locked in by the
        // integration test in
        // event_decoding_integration_test.dart.
        expect(
          () => ReasoningEncryptedValueEvent.fromJson({
            'type': 'REASONING_ENCRYPTED_VALUE',
            'subtype': 'bogus',
            'entityId': 'rsn_01',
            'encryptedValue': 'cipher',
          }),
          throwsA(isA<AGUIValidationError>()),
        );
      });

      test('Reasoning events dispatch via BaseEvent.fromJson', () {
        final cases = <Map<String, dynamic>, Type>{
          {'type': 'REASONING_START', 'messageId': 'm'}:
              ReasoningStartEvent,
          {
            'type': 'REASONING_MESSAGE_START',
            'messageId': 'm',
            'role': 'reasoning',
          }: ReasoningMessageStartEvent,
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