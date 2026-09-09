import 'dart:convert';
import 'dart:io';

import 'package:ag_ui/ag_ui.dart';
import 'package:test/test.dart';

// These legacy variants are intentionally exercised by the compatibility
// probe; their deprecation diagnostics are expected here.
// ignore_for_file: deprecated_member_use_from_same_package

// These switches intentionally have no default arm. They are compile probes:
// adding an EventType or a concrete BaseEvent requires an explicit migration
// here, making the approved three-event source change visible to consumers.
String _eventTypeName(EventType type) => switch (type) {
      EventType.textMessageStart => 'TextMessageStartEvent',
      EventType.textMessageContent => 'TextMessageContentEvent',
      EventType.textMessageEnd => 'TextMessageEndEvent',
      EventType.textMessageChunk => 'TextMessageChunkEvent',
      EventType.thinkingTextMessageStart => 'ThinkingTextMessageStartEvent',
      EventType.thinkingTextMessageContent => 'ThinkingTextMessageContentEvent',
      EventType.thinkingTextMessageEnd => 'ThinkingTextMessageEndEvent',
      EventType.toolCallStart => 'ToolCallStartEvent',
      EventType.toolCallArgs => 'ToolCallArgsEvent',
      EventType.toolCallEnd => 'ToolCallEndEvent',
      EventType.toolCallChunk => 'ToolCallChunkEvent',
      EventType.toolCallResult => 'ToolCallResultEvent',
      EventType.thinkingStart => 'ThinkingStartEvent',
      EventType.thinkingContent => 'ThinkingContentEvent',
      EventType.thinkingEnd => 'ThinkingEndEvent',
      EventType.stateSnapshot => 'StateSnapshotEvent',
      EventType.stateDelta => 'StateDeltaEvent',
      EventType.messagesSnapshot => 'MessagesSnapshotEvent',
      EventType.activitySnapshot => 'ActivitySnapshotEvent',
      EventType.activityDelta => 'ActivityDeltaEvent',
      EventType.raw => 'RawEvent',
      EventType.custom => 'CustomEvent',
      EventType.runStarted => 'RunStartedEvent',
      EventType.runFinished => 'RunFinishedEvent',
      EventType.runError => 'RunErrorEvent',
      EventType.stepStarted => 'StepStartedEvent',
      EventType.stepFinished => 'StepFinishedEvent',
      EventType.reasoningStart => 'ReasoningStartEvent',
      EventType.reasoningMessageStart => 'ReasoningMessageStartEvent',
      EventType.reasoningMessageContent => 'ReasoningMessageContentEvent',
      EventType.reasoningMessageEnd => 'ReasoningMessageEndEvent',
      EventType.reasoningMessageChunk => 'ReasoningMessageChunkEvent',
      EventType.reasoningEnd => 'ReasoningEndEvent',
      EventType.reasoningEncryptedValue => 'ReasoningEncryptedValueEvent',
    };

String _baseEventName(BaseEvent event) => switch (event) {
      TextMessageStartEvent() => 'TextMessageStartEvent',
      TextMessageContentEvent() => 'TextMessageContentEvent',
      TextMessageEndEvent() => 'TextMessageEndEvent',
      TextMessageChunkEvent() => 'TextMessageChunkEvent',
      ThinkingStartEvent() => 'ThinkingStartEvent',
      ThinkingContentEvent() => 'ThinkingContentEvent',
      ThinkingEndEvent() => 'ThinkingEndEvent',
      ThinkingTextMessageStartEvent() => 'ThinkingTextMessageStartEvent',
      ThinkingTextMessageContentEvent() => 'ThinkingTextMessageContentEvent',
      ThinkingTextMessageEndEvent() => 'ThinkingTextMessageEndEvent',
      ToolCallStartEvent() => 'ToolCallStartEvent',
      ToolCallArgsEvent() => 'ToolCallArgsEvent',
      ToolCallEndEvent() => 'ToolCallEndEvent',
      ToolCallChunkEvent() => 'ToolCallChunkEvent',
      ToolCallResultEvent() => 'ToolCallResultEvent',
      StateSnapshotEvent() => 'StateSnapshotEvent',
      StateDeltaEvent() => 'StateDeltaEvent',
      MessagesSnapshotEvent() => 'MessagesSnapshotEvent',
      ActivitySnapshotEvent() => 'ActivitySnapshotEvent',
      ActivityDeltaEvent() => 'ActivityDeltaEvent',
      RawEvent() => 'RawEvent',
      CustomEvent() => 'CustomEvent',
      RunStartedEvent() => 'RunStartedEvent',
      RunFinishedEvent() => 'RunFinishedEvent',
      RunErrorEvent() => 'RunErrorEvent',
      StepStartedEvent() => 'StepStartedEvent',
      StepFinishedEvent() => 'StepFinishedEvent',
      ReasoningStartEvent() => 'ReasoningStartEvent',
      ReasoningMessageStartEvent() => 'ReasoningMessageStartEvent',
      ReasoningMessageContentEvent() => 'ReasoningMessageContentEvent',
      ReasoningMessageEndEvent() => 'ReasoningMessageEndEvent',
      ReasoningMessageChunkEvent() => 'ReasoningMessageChunkEvent',
      ReasoningEndEvent() => 'ReasoningEndEvent',
      ReasoningEncryptedValueEvent() => 'ReasoningEncryptedValueEvent',
    };

String? _messageId(Message message) => message.id;

final _fixture = jsonDecode(
  File('test/fixtures/compatibility.json').readAsStringSync(),
) as Map<String, dynamic>;

void main() {
  group('compatibility baseline', () {
    test('fixture records the pinned base and all current variants', () {
      expect(_fixture['version'], 1);
      expect(
        _fixture['baseCommit'],
        'aaa75b54d572be8cd1d51c72e951273c5b893ed0',
      );
      expect((_fixture['eventTypes'] as List).length, 34);
      expect((_fixture['messageRoles'] as List).length, 7);
      expect((_fixture['cases'] as List).length, 14);
    });

    test('public package import and version remain available', () {
      expect(agUiVersion, '0.3.0');
      expect(initAgUI, returnsNormally);
    });

    test('const construction and constructor tear-offs remain valid', () {
      const message = UserMessage.fromContent(
        id: 'u1',
        messageContent: TextContent('hello'),
      );
      expect(message.content, 'hello');

      const UserMessage Function({required String id, required String content})
          userConstructor = UserMessage.new;
      final constructed = userConstructor(id: 'u2', content: 'tear-off');
      expect(constructed.content, 'tear-off');
      expect(message.copyWith().content, 'hello');
    });

    test('nullable Message.id typing and sentinel clearing are preserved', () {
      const message = UserMessage.fromContent(
        id: 'u1',
        messageContent: TextContent('hello'),
        name: 'named',
        encryptedValue: 'cipher',
      );
      final id = _messageId(message);
      expect(id, 'u1');
      expect(message.copyWith().name, 'named');
      expect(message.copyWith(name: null).name, isNull);
      expect(message.copyWith(encryptedValue: null).encryptedValue, isNull);
    });

    test('roles, defaults, and unknown fields follow the base contract', () {
      expect(
        MessageRole.values.map((role) => role.value),
        (_fixture['messageRoles'] as List).cast<String>(),
      );
      expect(
        const TextMessageStartEvent(messageId: 'm').role,
        TextMessageRole.assistant,
      );
      expect(
        const ReasoningMessageStartEvent(messageId: 'm').role,
        ReasoningMessageRole.reasoning,
      );

      final event = TextMessageStartEvent.fromJson({
        'type': 'TEXT_MESSAGE_START',
        'messageId': 'm',
        'futureField': {'kept': true},
      });
      expect(event.role, TextMessageRole.assistant);
      expect(event.toJson().containsKey('futureField'), isFalse);
    });

    test('arbitrary JSON and explicit nulls retain established behavior', () {
      final state = {
        'nested': [
          1,
          true,
          null,
          {'value': 'x'},
        ],
      };
      final snapshot = StateSnapshotEvent(snapshot: state);
      expect(StateSnapshotEvent.fromJson(snapshot.toJson()).snapshot, state);

      const raw = RawEvent(event: null);
      expect(raw.toJson().containsKey('event'), isTrue);
      expect(raw.toJson()['event'], isNull);

      final finished = RunFinishedEvent.fromJson({
        'type': 'RUN_FINISHED',
        'threadId': 't',
        'runId': 'r',
        'result': null,
      });
      expect(finished.result, isNull);
      expect(finished.toJson().containsKey('result'), isFalse);
    });

    test('cipher-bearing snapshots scrub rawEvent before re-emission', () {
      final cipher = {
        'type': 'MESSAGES_SNAPSHOT',
        'messages': [
          {
            'id': 'm',
            'role': 'assistant',
            'content': 'visible',
            'encryptedValue': 'secret',
          },
        ],
        'rawEvent': {'encryptedValue': 'secret'},
      };
      final snapshot = MessagesSnapshotEvent.fromJson(cipher);
      expect(snapshot.rawEvent, isNull);
      expect(snapshot.toJson().containsKey('rawEvent'), isFalse);
      expect(
        (snapshot.messages.single as AssistantMessage).encryptedValue,
        'secret',
      );

      final started = RunStartedEvent.fromJson({
        'type': 'RUN_STARTED',
        'threadId': 't',
        'runId': 'r',
        'input': {
          'threadId': 't',
          'runId': 'r',
          'messages': [
            {
              'id': 'm',
              'role': 'assistant',
              'content': 'visible',
              'encryptedValue': 'secret',
            },
          ],
          'tools': <Map<String, dynamic>>[],
          'context': <Map<String, dynamic>>[],
        },
        'rawEvent': {'encryptedValue': 'secret'},
      });
      expect(started.rawEvent, isNull);
    });

    test('Tool.metadata null is absent and explicitly clearable', () {
      final fromNull = Tool.fromJson({
        'name': 'search',
        'description': 'Search',
        'metadata': null,
      });
      expect(fromNull.metadata, isNull);
      expect(fromNull.toJson().containsKey('metadata'), isFalse);

      final withMetadata = fromNull.copyWith(metadata: {'source': 'test'});
      expect(withMetadata.metadata, {'source': 'test'});
      expect(withMetadata.copyWith().metadata, {'source': 'test'});
      expect(withMetadata.copyWith(metadata: null).metadata, isNull);
    });

    test('SimpleRunAgentInput keeps defaults and map assertions contract', () {
      const input = SimpleRunAgentInput();
      expect(input.toJson(), {
        'state': <String, dynamic>{},
        'messages': <Map<String, dynamic>>[],
        'tools': <Map<String, dynamic>>[],
        'context': <Map<String, dynamic>>[],
        'forwardedProps': <String, dynamic>{},
      });

      const populated = SimpleRunAgentInput(
        threadId: 't',
        runId: 'r',
        parentRunId: 'p',
        state: {'count': 1},
        forwardedProps: {'trace': true},
        config: {'model': 'test'},
        metadata: {'source': 'compatibility'},
      );
      expect(populated.toJson()['state'], {'count': 1});
      expect(populated.toJson()['forwardedProps'], {'trace': true});
      expect(populated.toJson()['config'], {'model': 'test'});
      expect(populated.toJson()['metadata'], {'source': 'compatibility'});
    });

    test('RunAgentInput preserves JSON, aliases, defaults, and clearing', () {
      const input = RunAgentInput(
        threadId: 't',
        runId: 'r',
        state: {'count': 1},
        messages: <Message>[],
        tools: <Tool>[],
        context: <Context>[],
        forwardedProps: {'trace': true},
      );
      expect(
        RunAgentInput.fromJson({
          'thread_id': 't',
          'run_id': 'r',
          'state': {'count': 1},
          'messages': <Map<String, dynamic>>[],
          'tools': <Map<String, dynamic>>[],
          'context': <Map<String, dynamic>>[],
          'forwarded_props': {'trace': true},
          'unknown': 'ignored',
        }).toJson(),
        input.toJson(),
      );
      expect(input.copyWith(parentRunId: null).parentRunId, isNull);
      expect(input.copyWith(state: null).state, isNull);
      expect(input.copyWith(forwardedProps: null).forwardedProps, isNull);
      expect(input.copyWith().forwardedProps, {'trace': true});
    });

    test('exhaustive switches cover every current event type and subtype', () {
      final expectedTypes = (_fixture['eventTypes'] as List)
          .map((item) => (item as Map<String, dynamic>)['wire'] as String)
          .toList();
      expect(
        EventType.values.map((type) => type.value).toList(),
        expectedTypes,
      );

      final events = <BaseEvent>[
        const TextMessageStartEvent(messageId: 'm'),
        const TextMessageContentEvent(messageId: 'm', delta: ''),
        const TextMessageEndEvent(messageId: 'm'),
        const TextMessageChunkEvent(),
        const ThinkingStartEvent(),
        const ThinkingContentEvent(delta: ''),
        const ThinkingEndEvent(),
        const ThinkingTextMessageStartEvent(),
        const ThinkingTextMessageContentEvent(delta: ''),
        const ThinkingTextMessageEndEvent(),
        const ToolCallStartEvent(toolCallId: 'c', toolCallName: 'search'),
        const ToolCallArgsEvent(toolCallId: 'c', delta: ''),
        const ToolCallEndEvent(toolCallId: 'c'),
        const ToolCallChunkEvent(),
        const ToolCallResultEvent(
          messageId: 'm',
          toolCallId: 'c',
          content: '',
        ),
        const StateSnapshotEvent(snapshot: <String, dynamic>{}),
        const StateDeltaEvent(delta: <Map<String, dynamic>>[]),
        MessagesSnapshotEvent(messages: <Message>[]),
        const ActivitySnapshotEvent(
          messageId: 'm',
          activityType: 'task',
          content: <String, dynamic>{},
        ),
        const ActivityDeltaEvent(
          messageId: 'm',
          activityType: 'task',
          patch: <Map<String, dynamic>>[],
        ),
        const RawEvent(event: null),
        const CustomEvent(name: 'custom', value: null),
        RunStartedEvent(threadId: 't', runId: 'r'),
        const RunFinishedEvent(threadId: 't', runId: 'r'),
        const RunErrorEvent(message: 'error'),
        const StepStartedEvent(stepName: 'step'),
        const StepFinishedEvent(stepName: 'step'),
        const ReasoningStartEvent(messageId: 'm'),
        const ReasoningMessageStartEvent(messageId: 'm'),
        const ReasoningMessageContentEvent(messageId: 'm', delta: ''),
        const ReasoningMessageEndEvent(messageId: 'm'),
        const ReasoningMessageChunkEvent(),
        const ReasoningEndEvent(messageId: 'm'),
        const ReasoningEncryptedValueEvent(
          subtype: ReasoningEncryptedValueSubtype.toolCall,
          entityId: 'm',
          encryptedValue: 'secret',
        ),
      ];
      const expectedBaseEventNames = [
        'TextMessageStartEvent',
        'TextMessageContentEvent',
        'TextMessageEndEvent',
        'TextMessageChunkEvent',
        'ThinkingStartEvent',
        'ThinkingContentEvent',
        'ThinkingEndEvent',
        'ThinkingTextMessageStartEvent',
        'ThinkingTextMessageContentEvent',
        'ThinkingTextMessageEndEvent',
        'ToolCallStartEvent',
        'ToolCallArgsEvent',
        'ToolCallEndEvent',
        'ToolCallChunkEvent',
        'ToolCallResultEvent',
        'StateSnapshotEvent',
        'StateDeltaEvent',
        'MessagesSnapshotEvent',
        'ActivitySnapshotEvent',
        'ActivityDeltaEvent',
        'RawEvent',
        'CustomEvent',
        'RunStartedEvent',
        'RunFinishedEvent',
        'RunErrorEvent',
        'StepStartedEvent',
        'StepFinishedEvent',
        'ReasoningStartEvent',
        'ReasoningMessageStartEvent',
        'ReasoningMessageContentEvent',
        'ReasoningMessageEndEvent',
        'ReasoningMessageChunkEvent',
        'ReasoningEndEvent',
        'ReasoningEncryptedValueEvent',
      ];
      expect(events.map(_baseEventName).toList(), expectedBaseEventNames);
      expect(EventType.values.map(_eventTypeName).length, 34);
    });
  });
}
